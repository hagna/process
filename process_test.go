package process

import (
	"testing"
	"io/ioutil"
	"os"
)

func TestBasic(t *testing.T) {
	contents := `#!/bin/sh

echo "hello there"
echo "hello cat"
`
	confirmOutput := func (contents string, output []string) {
		fname := "barbar"
		args := []string{"./barbar"}
		o := make(chan *Message)
		ioutil.WriteFile(fname, []byte(contents), 0777)	
		defer os.Remove(fname)
		go func() {
			i := 0 
			for j := range o {
				if i < len(output) {
				if j.Body != output[i] {
					t.Errorf("%s != %s", j.Body, output[i])
				} else {
					t.Errorf("%s == %s", j.Body, output[i])
				}
				i++
				} else {
					t.Errorf("too much output: \"%s\"", j.Body)
				}
			}
		}()
		p := StartProcess("", args, o)
		t.Log(p)
		<-p.Done
	}
	confirmOutput(contents, []string{"hello there\nhello cat"})
}
