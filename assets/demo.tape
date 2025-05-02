Require loog
Require tmux
Require kubectl

Output demo.mp4
Output demo.gif

Set FontSize 12
Set Height 720
Set Width 1280

Set Margin 0
Set Padding 10

Hide
  Type@0ms "tmux -f /dev/null -L test new-session -- zsh" Enter
  Type@0ms "tmux split-window -d -v -l 4 -- zsh && \" Enter
  Type@0ms "tmux set status && \" Enter
  Type@0ms 'tmux setw pane-border-style "fg=0" && \' Enter
  Type@0ms 'tmux setw pane-active-border-style "fg=0"' Enter
  Sleep 0.5
  Ctrl+L

  # full screen
  Ctrl+B
  Type z

  Sleep 1
Show

Type `loog -resource v1/namespaces -filter-expr 'Names("demo-test")'`
Sleep 0.5
Enter

# wait for the "WHERE REVISION" monkey
Wait+Screen /WHERE/
Wait+Screen /REVISION/

Sleep 1

# we are now ready to create the namespace
Ctrl+B
Type z # exit full screen
Ctrl+B
Type o # swich pane
Sleep 1
Type `kubectl create namespace demo-test`
Enter
Sleep 1
Ctrl+B
Type o # switch pane

# wait until the namespace is here
Wait+Screen /now/
Sleep 2
Right
Down
Sleep 1
Right
Sleep 0.5
Down

Sleep 5

# now edit the namespace
Ctrl+B
Type o
Type `kubectl annotate namespace demo-test hello="world"`
Enter
Sleep 1
Ctrl+B
Type o
Ctrl+B
Type z
Sleep 0.5

Wait+Screen /now/
Sleep 1
Down
Sleep 1
Type "pp"
Sleep 5
Type "pp"
Sleep 1
Type "h"
Sleep 8 

# Cleanup
Hide
  Type "q"
  Sleep 1

  Ctrl+B
  Type z
  Ctrl+B
  Type o
  Type `kubectl delete namespace demo-test`
  Enter
  Ctrl+B
  Type o
