set -e
./gen.sh
go run build.go
./ebiten
