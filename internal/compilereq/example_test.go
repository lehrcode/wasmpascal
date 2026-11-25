package compilereq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lehrcode/wasmpascal/internal/compilereq"
)

// Eine Anfrage lesen, übersetzen und die Antwort als JSON-Zeile schreiben,
// wie es pascal serve je Eingabezeile tut.
func Example() {
	line := []byte(`{"id": 1, "files": {"p.pas": "program P; begin x := 1 end."}}`)
	req, err := compilereq.Decode(line)
	var resp compilereq.Response
	if err != nil {
		resp = compilereq.RequestError(req.ID, err)
	} else {
		resp = compilereq.Handle(context.Background(), req)
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	fmt.Println(resp.Kind)
	// Output:
	// {"id":1,"kind":"source","error":"p.pas(1,18): Error 3: Unknown identifier (x)","file":"p.pas","line":1,"column":18,"code":3}
	// source
}
