package main

import (
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("go", "test", "./internal/payouts/", "-count=1")
	cmd.Dir = "/Users/dr.sazzadkhan/Downloads/Dev/ridex-angola-go"
	out, _ := cmd.CombinedOutput()
	os.WriteFile("/Users/dr.sazzadkhan/Downloads/Dev/ridex-angola-go/test_output.txt", out, 0644)
}
