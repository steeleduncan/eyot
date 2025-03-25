# Source me...
#
# A fairly minimal bash environment
# It is sourced by default with `nix develop`

ScriptDir=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
export EyotRoot="$ScriptDir"/../lib > /dev/null

function eyot {
    local EyotBinary=/tmp/eyot-binary

	pushd "$ScriptDir"/../src > /dev/null
	go build -o $EyotBinary eyot/cmd || {
        popd > /dev/null
        return 1
    }
	popd > /dev/null

	$EyotBinary $@
}

TempBinary="/tmp/eyot-test-binary"
if [ ! -z $TMP ]; then
    TempBinary="$TMP/eyot-test-binary"
fi

# pass a folder to run on (will run in local if not)
function eyot_test_folder {
    local RunFolder="$1"
    if [[ "$1" != /* ]]; then
        RunFolder=$(pwd)/"$1"
    fi
    echo "Test folder $RunFolder"

	pushd "$ScriptDir"/../src > /dev/null
    go build -o $TempBinary eyot/cmd || {
        echo "Binary build failed"
    }
	go run eyot/testcmd $TempBinary "$RunFolder" || {
		echo "Go tests failed"
		popd > /dev/null
		return 1
	}

	echo "Tests in folder $1 passed!"
	popd > /dev/null
    
}

BuildRoot="$TMP"
if [[ -z "$BuildRoot" ]]; then
    BuildRoot="/tmp"
fi
BuildFile="$BuildRoot/eyot-test-executable"

function eyot_grind_runtime {
    local Flags="-g -Wstrict-prototypes -Wall -Wextra -Wno-unused-function -DEYOT_OPENCL_INCLUDED -DEYOT_RUNTIME_MAX_ARGS=10"
    local LinkerFlags=""
    local Compiler=""

    if [[ "$OSTYPE" == "darwin"* ]]; then
        Flags="-framework OpenCL -DAppleBuild=1 $Flags"
        Compiler="$CC"
        if [ -z "$Compiler" ]; then
            echo "No CC set, falling back to clang"
            Compiler="clang"
        fi
    else
        LinkerFlags="-lOpenCL"
        Compiler="$CC"
        if [ -z "$Compiler" ]; then
            echo "No CC set, falling back to gcc"
            Compiler="gcc"
        fi
    fi

    local CompilerIdent="$($Compiler --version)"
    if [[ "$CompilerIdent" == *"clang"* ]]; then
        Flags="-Wno-unused-command-line-argument $Flags"
    else
        Flags="-fmax-errors=3 -Wno-unused-result -Werror $Flags"
    fi

    rm -f $BuildFile

	pushd "$ScriptDir"/../lib/runtime > /dev/null
    $Compiler $Flags *.c -o $BuildFile $LinkerFlags || {
		echo "Test build failed"
		popd > /dev/null
        return 1
    }
    popd > /dev/null

	valgrind $BuildFile || {
		echo "Test run failed ($BuildFile)"
		return 1
	}
}

function eyot_test_runtime {
    local Flags="-g -Wstrict-prototypes -Wall -Wextra -Wno-unused-function -DEYOT_OPENCL_INCLUDED -DEYOT_RUNTIME_MAX_ARGS=10"
    local LinkerFlags=""
    local Compiler=""

    if [[ "$OSTYPE" == "darwin"* ]]; then
        Flags="-framework OpenCL -DAppleBuild=1 $Flags"
        Compiler="$CC"
        if [ -z "$Compiler" ]; then
            echo "No CC set, falling back to clang"
            Compiler="clang"
        fi
    else
        LinkerFlags="-lOpenCL"
        Compiler="$CC"
        if [ -z "$Compiler" ]; then
            echo "No CC set, falling back to gcc"
            Compiler="gcc"
        fi
    fi

    local CompilerIdent="$($Compiler --version)"
    if [[ "$CompilerIdent" == *"clang"* ]]; then
        Flags="-Wno-unused-command-line-argument $Flags"
    else
        Flags="-fmax-errors=3 -fanalyzer -Wno-unused-result -Wno-analyzer-malloc-leak -Werror $Flags"
    fi

    rm -f $BuildFile

	pushd "$ScriptDir"/../lib/runtime > /dev/null
    $Compiler $Flags *.c -o $BuildFile $LinkerFlags || {
		echo "Test build failed"
		popd > /dev/null
        return 1
    }
    popd > /dev/null

	$BuildFile || {
		echo "Test run failed ($BuildFile)"
		return 1
	}
}

function eyot_test {
	eyot_test_runtime || {
		echo "Runtime tests failed"
		return 1
	}

	pushd "$ScriptDir"/../src > /dev/null

    # go level tests
	go test -v ./... || {
		echo "Go tests failed"
		popd > /dev/null
		return 1
	}

    # eyot level tests
    go build -o $TempBinary eyot/cmd || {
        echo "Binary build failed"
    }
	go run eyot/testcmd $TempBinary "$ScriptDir"/../tests || {
		echo "Go tests failed"
		popd > /dev/null
		return 1
	}

	echo "All tests passed!"
	popd > /dev/null
}

function eyot_reformat {
	pushd "$ScriptDir"/../lib/runtime > /dev/null
	clang-format -i *.c
	clang-format -i *.h
	popd > /dev/null
	pushd "$ScriptDir"/.. > /dev/null
	find src -name "*.go" | xargs gofmt -w
	popd > /dev/null
}
