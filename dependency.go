package gosupplychain

//import ("github.com/golang/gddo/gosrc"
//	"net/http"
//)
import (
	"log"
	"net/http"
	"net/mail"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/client9/gosupplychain/golist"

	"golang.org/x/tools/go/vcs"

	"github.com/ryanuber/go-license"
)

// Project contains VCS project data
// Notes:
//  go-source meta tag:  https://github.com/golang/gddo/wiki/Source-Code-Links
//     https://github.com/golang/gddo/blob/master/gosrc/gosrc.go
//
// Project contains an amalgamation of package, commit, repo, and license information
type Project struct {
	VcsName     string
	VcsCmd      string
	Repo        string
	LicenseLink string
}

// Commit contains meta data about a single commit
/*
type Commit struct {
	Rev     string
	Author  string
	Date    string
	Subject string
}
*/

//'{%n  "Commit": "%H",%n  "Author": "%an <%ae>",%n  "Date": "%ad",%n  "Message": "%f"%n},

// Dependency contains meta data on a external dependency
type Dependency struct {
	golist.Package
	Commit  Commit
	License license.License
	Project Project
}

// LinkToFile returns a URL that links to particular revision of a
// file or empty
//
func LinkToFile(pkg, file, rev string) string {
	if file == "" {
		return ""
	}

	switch {
	case strings.HasPrefix(pkg, "github.com"):
		if rev == "" {
			rev = "master"
		}
		return "https://" + pkg + "/blob/" + rev + "/" + file
	case strings.HasPrefix(pkg, "golang.org/x/"):
		if rev == "" {
			rev = "master"
		}
		return "https://github.com/golang/" + pkg[13:] + "/blob/" + rev + "/" + file
	case strings.HasPrefix(pkg, "gopkg.in"):
		return GoPkgInToGitHub(pkg) + "/" + file
	default:
		projectURL := "https://" + pkg
		resp, err := http.Get(projectURL + "?go-get=1")
		if err != nil {
			return projectURL
		}
		defer resp.Body.Close()
		_, mgs, err := parseMetaGo(resp.Body)
		if err != nil || mgs == nil {
			return projectURL
		}
		return mgs.FileURL("/", file)
	}
}

// GoPkgInToGitHub converts a "gopkg.in" to a github repo link
func GoPkgInToGitHub(name string) string {
	dir := ""
	var pkgversion string
	var user string
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return ""
	}
	if parts[0] != "gopkg.in" {
		return ""
	}

	idx := strings.Index(parts[1], ".")
	if idx != -1 {
		pkgversion = parts[1]
		if len(parts) > 2 {
			dir = "/" + strings.Join(parts[2:], "/")
		}
	} else {
		user = parts[1]
		pkgversion = parts[2]
		if len(parts) > 3 {
			dir = "/" + strings.Join(parts[3:], "/")
		}
	}
	idx = strings.Index(pkgversion, ".")
	if idx == -1 {
		return ""
	}
	pkg := pkgversion[:idx]
	if user == "" {
		user = "go-" + pkg
	}
	version := pkgversion[idx+1:]
	if version == "v0" {
		version = "master"
	}
	return "https://github.com/" + user + "/" + pkg + "/blob/" + version + dir
}

// GitCommitsBehind counts the number of commits a directory is behind master
func GitCommitsBehind(dir string, hash string) (int, error) {
	/*
		cmd := exec.Command("git", "fetch")
		_, err := cmd.Output()
		if err != nil {
			return -1, err
		}
	*/

	// the following doesn't work sometimes
	//cmd := exec.Command("git", "rev-list", "..master")
	cmd := exec.Command("git", "rev-list", "--count", "origin/master..."+hash)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// GetLastCommit returns meta data on the last commit
func GetLastCommit(dir string) (Commit, error) {
	log.Printf("Directory is %s", dir)
	cmd := exec.Command("git", "log", "-1", "--format=Commit: %H%nDate: %aD%nSubject: %s%n%n%b%n")
	cmd.Dir = dir
	msg, err := cmd.Output()
	if err != nil {
		log.Printf("git log error: %s", err)
		return Commit{}, err
	}
	//log.Printf("GOT %s", string(msg))
	r := strings.NewReader(string(msg))
	m, err := mail.ReadMessage(r)
	if err != nil {
		log.Printf("git log parse error: %s", err)
		return Commit{}, err
	}
	header := m.Header

	return Commit{
		Date:    header.Get("Date"),
		Commit:  header.Get("Commit"),
		Message: header.Get("Subject"),
	}, nil
}

// GetLicense returns licensing info
func GetLicense(path string) license.License {

	l, err := license.NewFromDir(path)
	if err != nil {
		return license.License{}
	}
	l.File = filepath.Base(l.File)
	return *l
}

// LoadDependencies is not done
func LoadDependencies(pkgs []string, ignores []string) ([]Dependency, error) {

	stdlib, err := golist.Std()
	if err != nil {
		return nil, err
	}

	pkgs, err = golist.Deps(pkgs...)
	if err != nil {
		return nil, err
	}

	// faster to remove stdlib
	pkgs = removeIfEquals(pkgs, stdlib)
	pkgs = removeIfSubstring(pkgs, ignores)
	deps, err := golist.Packages(pkgs...)
	if err != nil {
		return nil, err
	}

	visited := make(map[string]string, len(deps))

	out := make([]Dependency, 0, len(deps))
	for _, v := range deps {
		src := filepath.Join(v.Root, "src")
		path := filepath.Join(src, filepath.FromSlash(v.ImportPath))
		cmd, _, err := vcs.FromDir(path, src)
		if err != nil {
			log.Printf("error computing vcs %s: %s", path, err)
			continue
		}
		rr, err := vcs.RepoRootForImportPath(v.ImportPath, false)
		if err != nil {
			log.Printf("error computing repo for %s: %s", v.ImportPath, err)
			continue
		}
		e := Dependency{
			Package: v,
		}
		e.Project.Repo = rr.Repo
		e.Project.VcsName = cmd.Name
		e.Project.VcsCmd = cmd.Cmd
		e.License = GetLicense(path)
		visited[v.ImportPath] = e.License.Type

		// if child have no license, parent has license, continue
		// if child has no license, parent has no license, carry on
		if e.License.Type == "" {
			lic, ok := visited[rr.Root]
			if ok && lic != "" {
				// case is child has no license, but parent does
				// => just ignore this package
				continue
			}
			// if ok && lic = "" => don't look up parent

			if !ok {
				// first time checking parent
				parentpkg, err := golist.GetPackage(rr.Root)
				if err == nil {
					parent := Dependency{
						Package: parentpkg,
						Project: e.Project,
					}
					path = filepath.Join(src, filepath.FromSlash(rr.Root))
					parent.License = GetLicense(path)
					visited[rr.Root] = parent.License.Type
					if parent.License.Type != "" {
						// case child has no license, parent does
						//  ignore child and use parent
						e = parent
					}
				}
			}
		}
		commit, err := GetLastCommit(path)
		if err == nil {
			e.Commit = commit
		}

		e.Project.LicenseLink = LinkToFile(e.ImportPath, e.License.File, e.Commit.Commit)

		out = append(out, e)
	}
	return out, err
}

// generic []string function, remove elements of A that are in B
func removeIfEquals(alist []string, blist []string) []string {
	out := make([]string, 0, len(alist))
	for _, a := range alist {
		add := true
		for _, b := range blist {
			if a == b {
				add = false
			}
		}
		if add {
			out = append(out, a)
		}
	}
	return out
}

// removes elements of A that substring match any of B
func removeIfSubstring(alist []string, blist []string) []string {
	out := make([]string, 0, len(alist))
	for _, a := range alist {
		add := true
		for _, b := range blist {
			if strings.Index(a, b) != -1 {
				add = false
			}
		}
		if add {
			out = append(out, a)
		}
	}
	return out
}
