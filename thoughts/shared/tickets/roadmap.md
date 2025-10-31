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

[ ] Columns with bad sizing on configmaps
[ ] Columns with bad sizing on daemonsets
[ ] Columns with bad sizing on cronjobs
[ ] Columns with bad sizing on crds
[ ] Pageup/down hides the selected row
[ ] When filtering, the selected row is not always visible
[ ] Show the number of items e.g. Pods (50)
[ ] The filtered search is not sorted

[ ] HPAs cannot do yaml

[ ] Better error handling with multiple -context flags if we fail to connect to one of the clusters
[ ] If cannot connect to cluster, the connecting to API Server is always spinning

[X] Failed to refresh Consumer: informer not registered for jetstream.nats.io/v1beta2, Resource=consumers
[X] CRDs are not sorted
