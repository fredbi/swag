#! /bin/bash
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
cd "${root}"
mapfile ALL_MODULES < <(go list -f '{{.Dir}}' -m)
version="$(git rev-parse HEAD)"

mkdir -p sbom
rm -f sbom/*.json

for module_location in ${ALL_MODULES[@]} ;do
  name=$(go list)

  relative_location=${module_location#"$root"/}
  relative_location=${relative_location#"$root"}
  module_dir=${relative_location%"/go.mod"}
  base="${module_dir#"./"}"
  if [[ "${base}" ==  "" ]] ; then
    module="swag"
  else
    module="${base}"
  fi
  if [[ -z "${relative_location}" ]] ; then
    relative_location="."
  fi
  module="${module//\//-}"

  set -x
  syft scan \
    --config="${root}/.syft.yaml" --quiet=true \
    --source-version="${version}" \
    --source-name="${name}" \
    --output spdx-json="sbom/sbom-${module}-spdx.json" \
    --output cyclonedx-json="sbom/sbom-${module}-cyclonedx.json" \
    dir:"${relative_location}"
  set +x
done

