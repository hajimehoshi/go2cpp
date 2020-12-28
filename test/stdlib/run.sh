set -e
lib=$1
echo "# Test $lib"
env GOOS=js GOARCH=wasm go test -c -o test.wasm $lib
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -include autogen -wasm test.wasm -namespace go2cpp_autogen
clang++ -O3 -Wall -std=c++14 -pthread -I. -o test -g *.cpp autogen/*.cpp
shift
wd=$PWD
(cd $(go list -f '{{.Dir}}' $lib) && $wd/test $*)
