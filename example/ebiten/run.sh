set -e
env GOOS=js GOARCH=wasm go build -tags example -o ebiten.wasm -trimpath -tags=example github.com/hajimehoshi/ebiten/v2/examples/rotate
rm -rf autogen
go run ../../cmd/gowasm2cpp -out autogen -include autogen -wasm ebiten.wasm -namespace go2cpp_autogen
go run build.go
./ebiten
