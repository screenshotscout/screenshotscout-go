package screenshotscout

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestECMAScriptNumberFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value float64
		want  string
	}{
		{value: 0, want: "0"},
		{value: -0.0, want: "0"},
		{value: 1.25, want: "1.25"},
		{value: 0.000001, want: "0.000001"},
		{value: 0.0000001, want: "1e-7"},
		{value: 100000000000000000000.0, want: "100000000000000000000"},
		{value: 1e21, want: "1e+21"},
		{value: -1.2e-7, want: "-1.2e-7"},
	}
	for _, test := range tests {
		if got := ecmaScriptNumberString(test.value); got != test.want {
			t.Errorf("ecmaScriptNumberString(%g) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestCanonicalQuerySortsNamesAndPreservesDuplicateOrder(t *testing.T) {
	t.Parallel()

	pairs := []wirePair{
		{name: "url", value: "https://example.com/a path?x=~*"},
		{name: "headers", value: "X-Test:one"},
		{name: "headers", value: "X-Test:two"},
		{name: "delay", value: "0"},
	}
	want := "access_key=ak_test&delay=0&headers=X-Test%3Aone&headers=X-Test%3Atwo&url=https%3A%2F%2Fexample.com%2Fa%20path%3Fx%3D~%2A"
	if got := buildCanonicalQuery(pairs, "ak_test"); got != want {
		t.Fatalf("canonical query = %q, want %q", got, want)
	}
}

// The same vectors are copied unchanged into the gateway and every SDK.
func TestCanonicalQueryAndSignatureMatchSharedVectors(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(filepath.Join("testdata", "signing-vectors.json"))
	if err != nil {
		t.Fatalf("read signing vectors: %v", err)
	}
	var file struct {
		Vectors []struct {
			Name       string     `json:"name"`
			AccessKey  string     `json:"access_key"`
			SecretKey  string     `json:"secret_key"`
			Params     [][]string `json:"params"`
			SignedText string     `json:"signed_text"`
			Signature  string     `json:"signature"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(contents, &file); err != nil {
		t.Fatalf("decode signing vectors: %v", err)
	}
	if len(file.Vectors) == 0 {
		t.Fatal("no signing vectors")
	}
	for _, vector := range file.Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			t.Parallel()
			pairs := make([]wirePair, 0, len(vector.Params))
			for _, param := range vector.Params {
				if len(param) != 2 {
					t.Fatalf("param is not a [name, value] pair: %q", param)
				}
				pairs = append(pairs, wirePair{name: param[0], value: param[1]})
			}
			if signedText := buildCanonicalQuery(pairs, vector.AccessKey); signedText != vector.SignedText {
				t.Fatalf("signed text = %q, want %q", signedText, vector.SignedText)
			}
			client, err := NewClient(vector.AccessKey, &ClientOptions{SecretKey: vector.SecretKey})
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if signature := client.createSignature(pairs); signature != vector.Signature {
				t.Fatalf("signature = %s, want %s", signature, vector.Signature)
			}
		})
	}
}

func TestNegativeZeroMatchesJavaScriptQueryAndJSONEncoding(t *testing.T) {
	t.Parallel()

	negativeZero := math.Copysign(0, -1)
	serialized, err := serializeCaptureOptions(
		"https://example.com",
		&CaptureOptions{PDFScale: &negativeZero},
	)
	if err != nil {
		t.Fatalf("serializeCaptureOptions: %v", err)
	}
	if got := encodeQueryPairs(serialized.pairs); !strings.HasSuffix(got, "&pdf_scale=0") {
		t.Fatalf("negative zero query was not normalized to JavaScript form: %s", got)
	}
	body, err := encodeJSONBody(serialized.body)
	if err != nil {
		t.Fatalf("encodeJSONBody: %v", err)
	}
	if !strings.Contains(string(body), `"pdf_scale":0`) {
		t.Fatalf("negative zero JSON was not normalized to JavaScript form: %s", body)
	}
}

func TestSerializationRejectsTextAndIntegersThatCannotBeRepresentedSafely(t *testing.T) {
	t.Parallel()

	invalidUTF8 := string([]byte{0xff})
	_, err := serializeCaptureOptions("https://example.com", &CaptureOptions{Country: &invalidUTF8})
	if !errors.Is(err, ErrSerialization) {
		t.Fatalf("expected serialization error for invalid UTF-8, got %v", err)
	}

	if strconv.IntSize < 64 {
		return
	}
	unsafe64 := maxJSONSafeInteger + 1
	unsafeInteger := int(unsafe64)
	_, err = serializeCaptureOptions("https://example.com", &CaptureOptions{ImageWidth: &unsafeInteger})
	if !errors.Is(err, ErrSerialization) {
		t.Fatalf("expected serialization error for unsafe integer, got %v", err)
	}
}
