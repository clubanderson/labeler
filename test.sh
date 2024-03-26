#!/bin/bash

toUppercase() {
    while IFS= read -r line; do
        if [[ $line == kind:* || $line == apiVersion:* || $line == "  name:"* ]]; then
            line=$(echo "$line" | tr '[:lower:]' '[:upper:]')
        fi
        echo "$line"
    done
}

DetectOriginalCommand() {
    history -d 2  # Delete the last command from history
}

main() {
    local originalCommand
    DetectOriginalCommand

    # Optionally, if you want to capture the output of the command
    originalCommand=$(history -1)
    echo "Original command: $originalCommand"

    if isInputFromPipe; then
        echo "Data is from pipe"
        cat - | toUppercase
    else
        if [ -z "$flags_filepath" ]; then
            echo "Please input a file"
            exit 1
        fi
        if ! [ -e "$flags_filepath" ]; then
            echo "The file provided does not exist"
            exit 1
        fi
        cat "$flags_filepath" | toUppercase
    fi
}

isInputFromPipe() {
    [ -p /dev/stdin ]
}

main "$@"

