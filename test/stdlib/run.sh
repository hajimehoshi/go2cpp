set -e
echo "# Test $1"
env GOOS=js GOARCH=wasm go test -c -o test.wasm -trimpath $1
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -wasm test.wasm -namespace go2cpp_autogen
clang++ -Wall -std=c++14 -I. -o test -g *.cpp autogen/*.cpp
shift
./test $*
