set -e
./gen.sh github.com/hajimehoshi/go-inovation
go run build.go
./ebiten
