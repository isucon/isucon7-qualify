#!/bin/bash

set -eux

cd $(dirname ${0})
BASENAME="ubuntu1604"
OVA_FILE="${BASENAME}.ova"

if [[ -f ${OVA_FILE} ]]; then
  echo >&2 "${OVA_FILE} is already exists."
  echo >&2 "Please remove ${OVA_FILE} and retry"
  exit 2
fi

vagrant up
vagrant halt
vboxmanage export --ovf20 -o ${OVA_FILE} ${BASENAME}
#vagrant destroy -f
vagrant destroy
