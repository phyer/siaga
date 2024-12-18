module github.com/phyer/siaga

replace github.com/phyer/siaga/modules => ./modules

go 1.21

require (
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/phyer/core v0.1.29
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.18.1 // indirect
	github.com/phyer/texus v0.0.0-20241207132635-0e7fb63f8196 // indirect
	github.com/phyer/v5sdkgo v0.1.4 // indirect
	golang.org/x/sys v0.13.0 // indirect
)
