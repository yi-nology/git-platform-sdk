package sdk

import "fmt"

// TestFunction has a potential issue
func TestFunction() {
    var data map[string]string
    data["key"] = "value" // nil map panic
    fmt.Println(data)
}
