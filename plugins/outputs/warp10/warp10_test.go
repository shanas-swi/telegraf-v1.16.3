package warp10

import (
	"fmt"
	"testing"

	"github.com/shanas-swi/telegraf-v1.16.3/testutil"
	"github.com/stretchr/testify/require"
)

type ErrorTest struct {
	Message  string
	Expected string
}

func TestWriteWarp10(t *testing.T) {
	w := Warp10{
		Prefix:  "unit.test",
		WarpURL: "http://localhost:8090",
		Token:   "WRITE",
	}

	payload := w.GenWarp10Payload(testutil.MockMetrics())
	require.Exactly(t, "1257894000000000// unit.testtest1.value{source=telegraf,tag1=value1} 1.000000\n", payload)
}

func TestHandleWarp10Error(t *testing.T) {
	w := Warp10{
		Prefix:  "unit.test",
		WarpURL: "http://localhost:8090",
		Token:   "WRITE",
	}
	tests := [...]*ErrorTest{
		{
			Message: `
			<html>
			<head>
			<meta http-equiv="Content-Type" content="text/html;charset=utf-8"/>
			<title>Error 500 io.warp10.script.WarpScriptException: Invalid token.</title>
			</head>
			<body><h2>HTTP ERROR 500</h2>
			<p>Problem accessing /api/v0/update. Reason:
			<pre>    io.warp10.script.WarpScriptException: Invalid token.</pre></p>
			</body>
			</html>
			`,
			Expected: fmt.Sprintf("Invalid token"),
		},
		{
			Message: `
			<html>
			<head>
			<meta http-equiv="Content-Type" content="text/html;charset=utf-8"/>
			<title>Error 500 io.warp10.script.WarpScriptException: Token Expired.</title>
			</head>
			<body><h2>HTTP ERROR 500</h2>
			<p>Problem accessing /api/v0/update. Reason:
			<pre>    io.warp10.script.WarpScriptException: Token Expired.</pre></p>
			</body>
			</html>
			`,
			Expected: fmt.Sprintf("Token Expired"),
		},
		{
			Message: `
			<html>
			<head>
			<meta http-equiv="Content-Type" content="text/html;charset=utf-8"/>
			<title>Error 500 io.warp10.script.WarpScriptException: Token revoked.</title>
			</head>
			<body><h2>HTTP ERROR 500</h2>
			<p>Problem accessing /api/v0/update. Reason:
			<pre>    io.warp10.script.WarpScriptException: Token revoked.</pre></p>
			</body>
			</html>
			`,
			Expected: fmt.Sprintf("Token revoked"),
		},
		{
			Message: `
			<html>
			<head>
			<meta http-equiv="Content-Type" content="text/html;charset=utf-8"/>
			<title>Error 500 io.warp10.script.WarpScriptException: Write token missing.</title>
			</head>
			<body><h2>HTTP ERROR 500</h2>
			<p>Problem accessing /api/v0/update. Reason:
			<pre>    io.warp10.script.WarpScriptException: Write token missing.</pre></p>
			</body>
			</html>
			`,
			Expected: "Write token missing",
		},
		{
			Message:  `<title>Error 503: server unavailable</title>`,
			Expected: "<title>Error 503: server unavailable</title>",
		},
	}

	for _, handledError := range tests {
		payload := w.HandleError(handledError.Message, 511)
		require.Exactly(t, handledError.Expected, payload)
	}

}
