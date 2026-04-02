#!/usr/bin/env bash

set -euo pipefail

PRISM_PSM_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Backward-compatible entrypoint: load the shared framework and command bridges.
# The framework owns the common command lifecycle; bridges only provide command-specific adapters.
source "${PRISM_PSM_LIB_DIR}/framework.sh"
prism_psm_load_bridges
prism_psm_load_command_configs
