set -e
env GOOS=js GOARCH=wasm go build -tags example -o helloworld.wasm -trimpath .
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -wasm helloworld.wasm -namespace go2cpp_autogen
gcc -c -Wall -std=c++14 -I. autogen/*.cpp
