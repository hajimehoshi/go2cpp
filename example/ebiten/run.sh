set -e
./gen.sh -tags=example github.com/hajimehoshi/go-inovation
go run build.go
./ebiten
