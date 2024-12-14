
## Init

mkdir submodules && cd submodules 
git submodule add baidu:/root/repos/go/okexV5Api okex
cd ../
git pull
git submodule init
git submodule update --force --recursive --init --remote
go mod tidy
go mod vendor


