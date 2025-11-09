#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

. "$SCRIPT_DIR/settings.sh"

echo "Building $APP_NAME version $APP_VERSION"

DIST_DIR="${PROJECT_ROOT}/.dist"
LOG_DIR="${DIST_DIR}/logs"
mkdir -p "${DIST_DIR}" "${LOG_DIR}"

for OS in "${OS_ARRAY[@]}"; do
  for ARCH in "${ARCH_ARRAY[@]}"; do
    FULL_NAME="${APP_NAME}_${APP_VERSION}.${OS}-${ARCH}"
    OUT_DIR="${DIST_DIR}/${FULL_NAME}"
    mkdir -p "${OUT_DIR}"

    BIN_NAME="${APP_NAME}"
    if [[ "${OS}" == "windows" ]]; then
      BIN_NAME="${APP_NAME}.exe"
    fi
    DIST_PATH="${OUT_DIR}/${BIN_NAME}"

    echo "Building ${DIST_PATH}"
    (
      pushd "${PROJECT_ROOT}" || exit
      if GOOS="${OS}" GOARCH="${ARCH}" go build -o "${DIST_PATH}" -ldflags="-X 'main.ProgramVersion=${APP_VERSION}'" >> "${LOG_DIR}/${APP_NAME}.build.log" 2>&1; then
        sha256sum "${DIST_PATH}" | awk '{print $1}' > "${DIST_DIR}/${FULL_NAME}.sum.txt"
      fi
      popd || exit
    )

    tar -C "${DIST_DIR}" -czvf "${DIST_DIR}/${FULL_NAME}.tar.tgz" "${FULL_NAME}" >> "${LOG_DIR}/${APP_NAME}.build.log" 2>&1
    echo "Packaged ${DIST_DIR}/${FULL_NAME}.tar.tgz"
  done
done

echo "Done. Artifacts in: ${DIST_DIR}"

