module github.com/phyer/siaga

replace (
	v5sdk_go/config => ../core/submodules/okex/config
	v5sdk_go/rest => ../core/submodules/okex/rest
	v5sdk_go/utils => ../core/submodules/okex/utils
	v5sdk_go/ws => ../core/submodules/okex/ws
)

go 1.21

require (
	github.com/bitly/go-simplejson v0.5.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/phyer/core v0.1.1
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.18.1 // indirect
	github.com/phyer/texus v0.0.0-20241207132635-0e7fb63f8196 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	v5sdk_go/config v0.0.0-00010101000000-000000000000 // indirect
	v5sdk_go/rest v0.0.0-00010101000000-000000000000 // indirect
	v5sdk_go/utils v0.0.0-00010101000000-000000000000 // indirect
	v5sdk_go/ws v0.0.0-00010101000000-000000000000 // indirect
)
