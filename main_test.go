package main

import (
    "testing"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type inputOutputStringTest struct {
    input string 
	expectedStr string
	expectedErr error
}

var unescapePathTest = []inputOutputStringTest{
    inputOutputStringTest{"%C2%A5%C2%B7%C2%A3/te%24t/", ReplaceSlashWithFilepathSeparator("/¥·£/te$t"), nil},
    inputOutputStringTest{"%C2%A5%C2%B7%C2%A3/te%24t", ReplaceSlashWithFilepathSeparator("/¥·£/te$t"), nil},
    inputOutputStringTest{"%C2%A5%C2%B7%C2%A3/these/get/removed/../../../te%24t//", ReplaceSlashWithFilepathSeparator("/¥·£/te$t"), nil},
    inputOutputStringTest{"%C2%A5%C2%B7 % C 2%A3/te%24t//", "", errors.New("invalid URL escape \"% C\"")},
}

func TestUnescapeAndCleanPath(t *testing.T) {
	for _, test := range unescapePathTest{
        if actualStr, actualErr := unescapeAndCleanPath(test.input); actualStr != test.expectedStr || fmt.Sprintf("%s", actualErr) != fmt.Sprintf("%s", test.expectedErr) {
            t.Errorf("Output string %s not equal to expected %s, or output error '%s' not equal to expected '%s'.", actualStr, test.expectedStr, actualErr, test.expectedErr)
        }
    }
}

var verifyTargetPathTest = []inputOutputStringTest{
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£/te$t"), ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£/te$t"), nil},
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£/te$t/../"), ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£"), nil},
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£/te$t/.."), ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£"), nil},
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£/te$t/../.."), ReplaceSlashWithFilepathSeparator("/mnt/data"), nil},
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/¥·£/../../te$t"), "", errors.New("failed to properly verify destination file path '\\mnt\\te$t'. filepath did not end up in the '\\mnt\\data' directory")},
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/3/4/5"), ReplaceSlashWithFilepathSeparator("/mnt/data/3/4/5"), nil},
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/3/4/5/"), ReplaceSlashWithFilepathSeparator("/mnt/data/3/4/5"), nil},
    inputOutputStringTest{ReplaceSlashWithFilepathSeparator("/mnt/data/3/4/5/6"), "", errors.New("destination file path '\\mnt\\data\\3\\4\\5\\6' is too long. directory depth should not exceed '5', was '6'")},
}

func TestCleanAndVerifyTargetPath(t *testing.T) {
	for _, test := range verifyTargetPathTest{
        if actualStr, actualErr := cleanAndVerifyTargetPath(test.input); actualStr != test.expectedStr || fmt.Sprintf("%s", actualErr) != fmt.Sprintf("%s", test.expectedErr) {
            t.Errorf("Output string %s not equal to expected %s, or output error '%s' not equal to expected '%s'.", actualStr, test.expectedStr, actualErr, test.expectedErr)
        }
    }
}

func ReplaceSlashWithFilepathSeparator(input string) (string) {
	return strings.Replace(input, "/", string(filepath.Separator), -1)
}