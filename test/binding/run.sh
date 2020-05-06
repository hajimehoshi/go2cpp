set -e
env GOOS=js GOARCH=wasm go test -c -o test.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -include autogen -wasm test.wasm -namespace go2cpp_autogen
clang++ -Wall -std=c++14 -pthread -I. -o test -g *.cpp autogen/*.cpp
./test $*
