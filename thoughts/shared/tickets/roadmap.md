# Roadmap for Next Features

## Urgent

1. Search on yaml
2. Save yaml
3. Basic edit using kubectl command generation
4. Sticky namespaces
5. Basic CRD support
6. :ns not implemented

## Important

1. Log streaming support
2. Support more resource types
3. Shortcut to expand/collapse a column with copy+paste of the value

## UX

1. Show something on the pod list if it is the first screen
2. Allow user to say which screen to show first

## Nice to have

1. AI assistant for generating kubectl commands

## Bugs

[X] Columns with bad sizing on configmaps (Fixed: Phase 1)
[X] Columns with bad sizing on daemonsets (Fixed: Phase 1)
[X] Columns with bad sizing on cronjobs (Fixed: Phase 1)
[X] Columns with bad sizing on crds (Fixed: Phase 1)
[X] Pageup/down hides the selected row (Fixed: Phase 2)
[X] When filtering, the selected row is not always visible (Fixed: Phase 2)
[X] Show the number of items e.g. Pods (50) (Fixed: Phase 1)
[X] The filtered search is not sorted (Fixed: Phase 3)

[X] HPAs cannot do yaml (Fixed: Phase 5 - also fixed ReplicaSets, PVCs, Ingresses, Endpoints)
[X] Missing Spec field in describe output (Fixed: Phase 5)

[ ] Better error handling with multiple -context flags if we fail to connect to one of the clusters
[ ] If cannot connect to cluster, the connecting to API Server is always spinning

[X] Failed to refresh Consumer: informer not registered for jetstream.nats.io/v1beta2, Resource=consumers
[X] CRDs are not sorted

[ ] Context fails and the error is not shown