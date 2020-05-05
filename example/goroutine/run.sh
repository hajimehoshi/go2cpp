set -e
env GOOS=js GOARCH=wasm go build -tags example -o goroutine.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -wasm goroutine.wasm -namespace go2cpp_autogen
clang++ -Wall -std=c++14 -I. -o goroutine -g *.cpp autogen/*.cpp
./goroutine
