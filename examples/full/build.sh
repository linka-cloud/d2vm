#!/usr/bin/env bash

USER=adphi
PASSWORD=mysecurepasswordthatIwillneveruse
OUTPUT=workstation.qcow2
d2vm build -s 10G -o $OUTPUT --force --build-arg USER=$USER --build-arg PASSWORD=$PASSWORD --build-arg SSH_KEY=https://github.com/$USER.keys .
