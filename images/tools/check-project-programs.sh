#!/bin/bash

# Syntax-check executable sources owned by one image project. Project test
# scripts call this helper so discovery stays consistent without moving
# ownership back into a workspace-wide test.

set -euo pipefail

project_dir="${1:?usage: check-project-programs.sh <project-directory>}"

mapfile -t program_sources < <(
    find "$project_dir" -type f \( -path '*/shell/*' -o -name 'init.sh' \
        -o -path '*/openbox/autostart' -o -path '*/kasm/*.sh' \
        -o -path '*/overlay/usr/local/bin/*' \
        -o -path '*/overlay/usr/local/lib/*' \
        -o -path '*/overlay/etc/desktop/*' -o -name 'cortilectl' \) \
        ! -path '*/tests/*' | sort
)

shell_sources=()
python_sources=()
for source in "${program_sources[@]}"; do
    if [[ "$source" == *.py ]] || head -c 64 "$source" | grep -q python; then
        python_sources+=("$source")
    else
        shell_sources+=("$source")
    fi
done

if [ "${#shell_sources[@]}" -gt 0 ]; then
    bash -n "${shell_sources[@]}"
fi

if [ "${#python_sources[@]}" -gt 0 ]; then
    pycache="$(mktemp -d)"
    PYTHONPYCACHEPREFIX="$pycache" python3 -m py_compile "${python_sources[@]}"
    rm -rf "$pycache"
fi

if find "$project_dir" -type d -name __pycache__ -print -quit | grep -q .; then
    echo "$project_dir contains __pycache__; it would enter the image context." >&2
    exit 1
fi
