set -e
env GOOS=js GOARCH=wasm go build -tags example -o helloworld.wasm -trimpath .
rm -rf autogen *.o
go run ../../cmd/gowasm2cpp -out autogen -wasm helloworld.wasm -namespace go2cpp_autogen
clang++ -Wall -std=c++14 -I. -o helloworld -g *.cpp autogen/*.cpp
./helloworld
