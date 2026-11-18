package lint

import "testing"

func TestParseKeepsDescriptionAndHTTPSLink(t *testing.T) {
	out := []byte(`{"stdin.md":[
	 {"Span":[1,3],"Check":"Vale.Spelling","Message":"Did you really mean 'Teh'?","Severity":"error","Match":"Teh","Line":1,
	  "Description":"Checks spelling.","Link":"https://vale.sh/docs/checks/spelling"},
	 {"Span":[5,9],"Check":"X.Y","Message":"m","Severity":"warning","Match":"title","Line":1,"Link":"javascript:alert(1)"}]}`)
	alerts, err := parse(out, []byte("Teh title"))
	if err != nil {
		t.Fatal(err)
	}
	if alerts[0].Description != "Checks spelling." || alerts[0].Link != "https://vale.sh/docs/checks/spelling" {
		t.Errorf("first: %+v", alerts[0])
	}
	if alerts[1].Link != "" {
		t.Errorf("non-https link kept: %q", alerts[1].Link)
	}
}
