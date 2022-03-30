#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname ${BASH_SOURCE})/../..

cd $REPO_ROOT

${REPO_ROOT}/bin/kustomize build config/variants/enterprise >deploy/single/all-in-one-dbless-enterprise.yaml
${REPO_ROOT}/bin/kustomize build config/variants/enterprise-postgres >deploy/single/all-in-one-postgres-enterprise.yaml
