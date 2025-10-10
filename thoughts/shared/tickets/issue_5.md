# Summary

Several improvements to context switching.

# Acceptance Criteria

1. I want do use ctrl+1, ctrl+2, etc. to switch between contexts.
2. if ctrl+n is more than 9, use ctrl+0 for 10, ctrl+- for :prev-context, ctrl+= for :next-context,
3. if the context pool does not have that many contexts, do nothing
4. in the contexts screen, show the shortcuts in a new column on the left
5. in the context screen, put the ones loaded/loading/error loaded first, then the rest in alphabetical order
6. in the context screen, include the status in the search
7. in the context screen, i want the fuzzy search to be less fuzzy, so that if i type "wo" it doesn't match "work" but does match "wo" or "woa" or "wok"
8. in the header, when showing messages about the context being loaded I replace by "Loading contexts 1/n: <context name>", where n is the total number of contexts and 1 is the number of contexts loaded so far
9. document context support (several flags, loading in the background, context switching, context screen and command) in the readme