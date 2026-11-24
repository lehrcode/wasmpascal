package rtl_test

import (
	"fmt"
	"io/fs"

	"github.com/lehrcode/wasmpascal/rtl"
)

func ExampleFS() {
	names, _ := fs.Glob(rtl.FS, "system.pas")
	fmt.Println(names)
	// Output: [system.pas]
}
