package main
import (
	"fmt"
	"github.com/gofiber/storage/redis/v3"
)
func main() {
	fmt.Printf("%+v\n", redis.Config{})
}
