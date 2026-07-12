package main
import (
	"fmt"
	"net/url"
)
func main() {
	u, _ := url.Parse("file://migrations")
	fmt.Printf("Host: %q, Path: %q\n", u.Host, u.Path)
	u2, _ := url.Parse("file:///migrations")
	fmt.Printf("Host: %q, Path: %q\n", u2.Host, u2.Path)
}
