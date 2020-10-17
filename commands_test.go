package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

/******************************************************************************
Oct, 15, 2020

Testing command line utilities via subroutines can be annoying so
if you're doing it from the commandline be sure to compile first.
From the project's root directory often use:

go build && go install && go test -v ./...

To accurately test your commands you MUST make sure to rebuild and reinstall
before you run your tests. Otherwise your system version will be out of
date and will give you results using an older build.

Happy hacking,
Tim


TODO:

write subtest to check for empty output before merge
******************************************************************************/

func TestConvert(t *testing.T) {

	if runtime.GOOS == "windows" {
		fmt.Println("TestConvert was not run and autopassed. Currently Poly command line support is not available for windows. See https://github.com/TimothyStiles/poly/issues/16.")
	} else {

		// testing redirected pipe output
		command := "cat data/bsub.gbk | poly c -i gbk -o json > data/converttest.json"
		exec.Command("bash", "-c", command).Output()

		// getting test sequence from non-pipe io to compare against redirected io
		baseTestSequence := ReadGbk("data/bsub.gbk")
		outputTestSequence := ReadJSON("data/converttest.json")

		// cleaning up test data
		os.Remove("data/converttest.json")

		// diff original sequence and converted sequence reread back into
		// AnnotatedSequence format. If there's an error fail test and print diff.
		if diff := cmp.Diff(baseTestSequence, outputTestSequence, cmpopts.IgnoreFields(Feature{}, "ParentAnnotatedSequence")); diff != "" {
			t.Errorf(" mismatch from convert pipe input test (-want +got):\n%s", diff)
		}

		//clearing possibly still existent prior test data.
		os.Remove("data/ecoli-mg1655.json")
		os.Remove("data/bsub.json")

		// testing multithreaded non piped output
		command = "poly c -o json data/bsub.gbk data/ecoli-mg1655.gff"
		exec.Command("bash", "-c", command).Output()

		ecoliInputTestSequence := ReadGff("data/ecoli-mg1655.gff")
		ecoliOutputTestSequence := ReadJSON("data/ecoli-mg1655.json")

		//clearing test data.
		os.Remove("data/ecoli-mg1655.json")

		// compared input gff from resulting output json. Fail test and print diff if error.
		if diff := cmp.Diff(ecoliInputTestSequence, ecoliOutputTestSequence, cmpopts.IgnoreFields(Feature{}, "ParentAnnotatedSequence")); diff != "" {
			t.Errorf(" mismatch from concurrent gff input test (-want +got):\n%s", diff)
		}

		bsubInputTestSequence := ReadGbk("data/bsub.gbk")
		bsubOutputTestSequence := ReadJSON("data/bsub.json")

		// clearing test data.
		os.Remove("data/bsub.json")

		// compared input gbk from resulting output json. Fail test and print diff if error.
		if diff := cmp.Diff(bsubInputTestSequence, bsubOutputTestSequence, cmpopts.IgnoreFields(Feature{}, "ParentAnnotatedSequence")); diff != "" {
			t.Errorf(" mismatch from concurrent gbk input test (-want +got):\n%s", diff)
		}

	}
}

func TestHash(t *testing.T) {
	if runtime.GOOS == "windows" {
		fmt.Println("TestHash was not run and autopassed. Currently Poly command line support is not available for windows. See https://github.com/TimothyStiles/poly/issues/16.")
	} else {

		puc19GbkBlake3Hash := "4b0616d1b3fc632e42d78521deb38b44fba95cca9fde159e01cd567fa996ceb9"

		// testing pipe input
		command := "cat data/puc19.gbk | poly hash -i gbk"
		hashOutput, _ := exec.Command("bash", "-c", command).Output()
		hashOutputString := strings.TrimSpace(string(hashOutput))

		if hashOutputString != puc19GbkBlake3Hash {
			t.Errorf("TestHash for piped input has failed. Returned %q, want %q", hashOutputString, puc19GbkBlake3Hash)
		}

		// testing regular input
		command = "poly hash data/puc19.gbk"
		hashOutput, _ = exec.Command("bash", "-c", command).Output()
		hashOutputString = strings.TrimSpace(string(hashOutput))

		if hashOutputString != puc19GbkBlake3Hash {
			t.Errorf("TestHash for regular input has failed. Returned %q, want %q", hashOutputString, puc19GbkBlake3Hash)
		}

		// testing json write output
		command = "poly hash -o json data/puc19.gbk"
		exec.Command("bash", "-c", command).Output()
		hashOutputString = ReadJSON("data/puc19.json").Sequence.Hash
		os.Remove("data/puc19.json")

		if hashOutputString != puc19GbkBlake3Hash {
			t.Errorf("TestHash for json write output has failed. Returned %q, want %q", hashOutputString, puc19GbkBlake3Hash)
		}

	}
}
