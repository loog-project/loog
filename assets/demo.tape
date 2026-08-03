# loog demo - records the MVP feature tour in simulate mode (no cluster needed).
# Regenerate with: ./assets/make-demo.sh  (builds loog, runs vhs, burns captions)
Require loog

Output demo-raw.mp4

Set Shell zsh
Set FontSize 13
Set Width 1400
Set Height 820
Set Padding 10
Set Margin 0
Set TypingSpeed 55ms

Hide
Type "clear" Enter
Ctrl+L
Show

# Launch in simulate mode
Type "loog --simulate"
Sleep 700ms
Enter
Sleep 4s

# Explorer: browse the resource tree down to a deployment with history
Type@280ms "jjjjjjjj"
Sleep 2500ms

# Revisions: scroll the history (the detail diff follows the selection)
Type "2"
Sleep 900ms
Type@500ms "jjj"
Sleep 2500ms

# Detail view modes: focus the detail panel, then Changes / Object / Diff
Type "3"
Sleep 900ms
Type "p"
Sleep 3s
Type "o"
Sleep 3s
Type "d"
Sleep 2500ms

# Command palette -> switch to the Timeline
Ctrl+K
Sleep 1500ms
Type@85ms "timeline"
Sleep 1500ms
Enter
Sleep 2500ms

# Timeline: navigate, then reverse the sort order and back
Type@430ms "jjjjj"
Sleep 2s
Type "R"
Sleep 2500ms
Type "R"
Sleep 1500ms

# Mark two revisions for comparison
Type "c"
Sleep 1200ms
Type@430ms "jjjj"
Sleep 800ms
Type "c"
Sleep 1200ms

# Command palette -> Compare view (side-by-side)
Ctrl+K
Sleep 1s
Type@85ms "to compare"
Sleep 1200ms
Enter
Sleep 3500ms

# Help overlay
Type "?"
Sleep 3500ms
Escape
Sleep 700ms

Hide
Type "q"
Sleep 500ms
