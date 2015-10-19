

build:
	go build ./...
	go vet ./...
	golint ./...
	find . -name '*.go' | xargs gofmt -w -s
	find . -name '*.go' | xargs goimports -w

reports/github-corporate.md: Makefile
	go run ./github-search/main.go att airbnb aws bitly cloudflare coreos datadog docker ebay elastic etsy \
		facebookgo fastly gilt \
		github google hashicorp heroku influxdb koding \
		microsoft netflix  paperlesspost pivotal-golang \
		samsung sendgrid sony soundcloud spotify \
		square stripe timehop \
		uber vimeo yahoo yelp > reports/github-corporate.md

corp: reports/github-corporate.md

reports/github-users.md: Makefile
	go run ./github-search/main.go \
		alecthomas \
		araddon	\
		BurntSushi \
		davecheney \
		fatih \
		mattn \
		miekg \
		mitchellh \
		tinylib \
		tj \
		philhofer \
		robertkrimen \
		robpike \
		ryanuber \
	> reports/github-users.md

reports/golang-contributors.md: Makefile
	go run ./github-search/main.go \
	  	golang \
		0intro 4ad aclements alexcesaro ality atom-symbol bradfitz c4milo campoy crawshaw \
		dsymonds dvyukov jpoirier marete minux mpvl nicolasgarnier rakyll rui314 wathiede \
	> reports/golang-contributors.md


clean:
	rm -f *~
	rm -f people.md corp.md
