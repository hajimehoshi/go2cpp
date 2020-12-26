set -e
echo "# Test $1"
env GOOS=js GOARCH=wasm go test -c -o test.wasm $1
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -include autogen -wasm test.wasm -namespace go2cpp_autogen
clang++ -O3 -Wall -std=c++14 -pthread -I. -o test -g *.cpp autogen/*.cpp
shift
./test $*
