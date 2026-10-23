package dotreg_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/libnumafly/dotReg/implementations/go"
)

func Example() {
	entries, err := dotreg.Parse(strings.NewReader(
		"@dotReg 1\n\nA.Code(string):001\nA.Count:42\nA.Missing:\nA.Empty:\"\"\n",
	))
	if err != nil {
		panic(err)
	}
	config, err := dotreg.Build(entries, dotreg.DemoProfile())
	if err != nil {
		panic(err)
	}
	code, err := config.Get([]string{"A", "Code"}, "string")
	if err != nil {
		panic(err)
	}
	count, err := config.Get([]string{"A", "Count"}, "int")
	if err != nil {
		panic(err)
	}
	missing, err := config.IsUnset([]string{"A", "Missing"})
	if err != nil {
		panic(err)
	}
	empty, err := config.IsUnset([]string{"A", "Empty"})
	if err != nil {
		panic(err)
	}
	fmt.Println(code)
	fmt.Println(count)
	fmt.Println(missing, empty)
	_, err = config.Get([]string{"A", "Code"}, "int")
	var mismatch *dotreg.Error
	if errors.As(err, &mismatch) {
		fmt.Println(mismatch.Code)
	}
	// Output:
	// 001
	// 42
	// true false
	// type_mismatch
}

func ExampleConfig_GetAll() {
	entries, err := dotreg.Parse(strings.NewReader(
		"@dotReg 1\n\nA.Tag:red\nA.Tag:green\nA.Tag:red\n",
	))
	if err != nil {
		panic(err)
	}
	config, err := dotreg.Build(entries, dotreg.DemoProfile())
	if err != nil {
		panic(err)
	}
	tags, err := config.GetAll([]string{"A", "Tag"}, "string")
	if err != nil {
		panic(err)
	}
	states, err := config.IsUnsetAll([]string{"A", "Tag"})
	if err != nil {
		panic(err)
	}
	second, err := config.GetAt([]string{"A", "Tag"}, 1, "string")
	if err != nil {
		panic(err)
	}
	fmt.Println(tags)
	fmt.Println(states)
	fmt.Println(second)
	_, err = config.Get([]string{"A", "Tag"}, "string")
	var multiple *dotreg.Error
	if errors.As(err, &multiple) {
		fmt.Println(multiple.Code, multiple.Line)
	}
	// Output:
	// [red green red]
	// [false false false]
	// green
	// multiple_values 4
}
