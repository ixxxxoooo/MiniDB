#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ENV="$PROJECT_ROOT/project.env"

if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>" >&2
    exit 1
fi

VERSION="${1#v}"

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Version must look like 1.2.3 or v1.2.3" >&2
    exit 1
fi

set -a
# shellcheck source=/dev/null
. "$PROJECT_ENV"
set +a
cd "$PROJECT_ROOT"

APP_VERSION="$VERSION"

escape_perl_replacement() {
    printf '%s' "$1" | sed 's/[\/&]/\\&/g'
}

set_env_var() {
    local key="$1"
    local value
    value="$(escape_perl_replacement "$2")"
    perl -0pi -e 's/^'"$key"'=.*/'"$key"'='"$value"'/m' "$PROJECT_ENV"
}

set_env_var APP_VERSION "$VERSION"

replace_json_string() {
    local key="$1"
    local value
    value="$(escape_perl_replacement "$2")"
    shift 2
    perl -0pi -e 's/"'"$key"'":\s*"[^"]+"/"'"$key"'": "'"$value"'"/g' "$@"
}

replace_yaml_string() {
    local key="$1"
    local value
    value="$(escape_perl_replacement "$2")"
    shift 2
    perl -0pi -e 's/('"$key"':\s*)"[^"]+"/${1}"'"$value"'"/g' "$@"
}

replace_plist_string() {
    local key="$1"
    local value
    value="$(escape_perl_replacement "$2")"
    shift 2
    perl -0pi -e 's/(<key>'"$key"'<\/key>\s*<string>)[^<]+(<\/string>)/${1}'"$value"'${2}/g' "$@"
}

replace_go_string() {
    local key="$1"
    local value
    value="$(escape_perl_replacement "$2")"
    perl -0pi -e 's/('"$key"'\s*=\s*")[^"]+(")/${1}'"$value"'${2}/' internal/version/version.go
}

replace_json_string version "$VERSION" wails.json
replace_json_string companyName "$APP_COMPANY_NAME" wails.json
replace_json_string productName "$APP_DISPLAY_NAME" wails.json
replace_json_string productIdentifier "$APP_PRODUCT_IDENTIFIER" wails.json
replace_json_string description "$APP_DESCRIPTION" wails.json
replace_json_string copyright "$APP_COPYRIGHT" wails.json
replace_json_string comments "$APP_COMMENTS" wails.json

perl -0pi -e 's/(info:\n(?:  [^\n]*\n)*?  version:\s*)"[^"]+"/${1}"'"$VERSION"'"/' build/config.yml
replace_yaml_string companyName "$APP_COMPANY_NAME" build/config.yml
replace_yaml_string productName "$APP_DISPLAY_NAME" build/config.yml
replace_yaml_string productIdentifier "$APP_PRODUCT_IDENTIFIER" build/config.yml
replace_yaml_string description "$APP_DESCRIPTION" build/config.yml
replace_yaml_string copyright "$APP_COPYRIGHT" build/config.yml
replace_yaml_string comments "$APP_COMMENTS" build/config.yml

replace_plist_string CFBundleName "$APP_DISPLAY_NAME" build/darwin/Info.plist build/darwin/Info.dev.plist
replace_plist_string CFBundleExecutable "$APP_BINARY_NAME" build/darwin/Info.plist build/darwin/Info.dev.plist
replace_plist_string CFBundleIdentifier "$APP_PRODUCT_IDENTIFIER" build/darwin/Info.plist build/darwin/Info.dev.plist
replace_plist_string CFBundleShortVersionString "$VERSION" build/darwin/Info.plist build/darwin/Info.dev.plist
replace_plist_string CFBundleVersion "$VERSION" build/darwin/Info.plist build/darwin/Info.dev.plist
replace_plist_string LSMinimumSystemVersion "$MACOS_MIN_VERSION_PLIST" build/darwin/Info.plist build/darwin/Info.dev.plist
replace_plist_string NSHumanReadableCopyright "$APP_COPYRIGHT" build/darwin/Info.plist build/darwin/Info.dev.plist

replace_json_string file_version "$VERSION" build/windows/info.json
replace_json_string ProductVersion "$VERSION" build/windows/info.json
replace_json_string CompanyName "$APP_COMPANY_NAME" build/windows/info.json
replace_json_string FileDescription "$APP_DESCRIPTION" build/windows/info.json
replace_json_string LegalCopyright "$APP_COPYRIGHT" build/windows/info.json
replace_json_string ProductName "$APP_DISPLAY_NAME" build/windows/info.json
perl -0pi -e 's/(<assemblyIdentity[^>]* name=")[^"]+(")/${1}'"$(escape_perl_replacement "$APP_PRODUCT_IDENTIFIER")"'${2}/' build/windows/wails.exe.manifest
perl -0pi -e 's/(<assemblyIdentity[^>]* version=")[^"]+(")/${1}'"$VERSION"'.0${2}/' build/windows/wails.exe.manifest

perl -0pi -e 's/(!define INFO_PROJECTNAME ")[^"]+(")/${1}'"$(escape_perl_replacement "$APP_BINARY_NAME")"'${2}/g' build/windows/nsis/wails_tools.nsh
perl -0pi -e 's/(!define INFO_COMPANYNAME ")[^"]+(")/${1}'"$(escape_perl_replacement "$APP_COMPANY_NAME")"'${2}/g' build/windows/nsis/wails_tools.nsh
perl -0pi -e 's/(!define INFO_PRODUCTNAME ")[^"]+(")/${1}'"$(escape_perl_replacement "$APP_DISPLAY_NAME")"'${2}/g' build/windows/nsis/wails_tools.nsh
perl -0pi -e 's/(!define INFO_PRODUCTVERSION ")[^"]+(")/${1}'"$VERSION"'${2}/g' build/windows/nsis/wails_tools.nsh
perl -0pi -e 's/(!define INFO_COPYRIGHT ")[^"]+(")/${1}'"$(escape_perl_replacement "$APP_COPYRIGHT")"'${2}/g' build/windows/nsis/wails_tools.nsh

replace_json_string name "$APP_BINARY_NAME" frontend/package.json
replace_json_string version "$VERSION" frontend/package.json
perl -0pi -e 's#<title>[^<]*</title>#<title>'"$(escape_perl_replacement "$APP_DISPLAY_NAME")"'</title>#' frontend/index.html

replace_go_string Version "$VERSION"
replace_go_string AppName "$APP_DISPLAY_NAME"
replace_go_string CompanyName "$APP_COMPANY_NAME"
replace_go_string Description "$APP_DESCRIPTION"
replace_go_string Repository "$APP_REPOSITORY"
replace_go_string DataDirName "$APP_DATA_DIR"

echo "Version set to $VERSION"
