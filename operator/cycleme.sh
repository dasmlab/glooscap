#!/bin/bash
#
# cycleme.sh - Cycles your Operator installation, container build and pusblish so you are always working on a CI/CD production like way.
#
#  Assumes you have set names and vars appropraitely.

# REMOVE OPERATOR AND BITS FIRST
make undeploy uninstall
make generate
make manifests

# Build a new version of the operatora nd publish it, bumpinhg SemVer
./buildme.sh
./pushme.sh

# Deploy CRDs to the Target Cluster (Assumes Kubeconfig is set properly, perms, etc)
make install deploy

# Create a Registry secret with your Token (pullSecret)
./create-registry-secret.sh
