#!/bin/bash

# ViSiON/3 BBS Color Chart - Shows actual ANSI colors as they appear in terminal

echo "=================================================="
echo "ViSiON/3 BBS Color Chart - ANSI Terminal Colors"
echo "=================================================="
echo

echo "FOREGROUND COLORS (|00 - |15):"
echo "--------------------------------"

# Low intensity colors (|00-|07)
echo -e "\\033[0;30m|00 Black (Dark)\\033[0m       \\033[1;30m|08 Dark Gray (Bright Black)\\033[0m"
echo -e "\\033[0;31m|01 Red (Dark)\\033[0m         \\033[1;31m|09 Bright Red\\033[0m"
echo -e "\\033[0;32m|02 Green (Dark)\\033[0m       \\033[1;32m|10 Bright Green\\033[0m"
echo -e "\\033[0;33m|03 Brown/Yellow (Dark)\\033[0m \\033[1;33m|11 Yellow (Bright)\\033[0m"
echo -e "\\033[0;34m|04 Blue (Dark)\\033[0m        \\033[1;34m|12 Bright Blue\\033[0m"
echo -e "\\033[0;35m|05 Magenta (Dark)\\033[0m     \\033[1;35m|13 Bright Magenta\\033[0m"
echo -e "\\033[0;36m|06 Cyan (Dark)\\033[0m        \\033[1;36m|14 Bright Cyan\\033[0m"
echo -e "\\033[0;37m|07 Gray (Light Gray)\\033[0m  \\033[1;37m|15 White (Bright White)\\033[0m"

echo
echo "BACKGROUND COLORS (|B0 - |B7):"
echo "--------------------------------"

echo -e "\\033[40m\\033[37m |B0 Black Background \\033[0m"
echo -e "\\033[41m\\033[37m |B1 Red Background   \\033[0m"
echo -e "\\033[42m\\033[30m |B2 Green Background \\033[0m"
echo -e "\\033[43m\\033[30m |B3 Brown Background \\033[0m"
echo -e "\\033[44m\\033[37m |B4 Blue Background  \\033[0m"
echo -e "\\033[45m\\033[37m |B5 Magenta Background \\033[0m"
echo -e "\\033[46m\\033[30m |B6 Cyan Background  \\033[0m"
echo -e "\\033[47m\\033[30m |B7 White Background \\033[0m"

echo
echo "COMBINATION EXAMPLES:"
echo "---------------------"

echo -e "Normal text: \\033[0;32mThis is |02 green text\\033[0m"
echo -e "Bright text: \\033[1;32mThis is |10 bright green text\\033[0m"
echo -e "Background:  \\033[44m\\033[1;37mThis is |15 white on |B4 blue background\\033[0m"
echo -e "Mixed:       \\033[1;33m\\033[41mThis is |11 yellow on |B1 red background\\033[0m"

echo
echo "CLASSIC BBS COMBINATIONS:"
echo "--------------------------"

echo -e "\\033[1;36m\\033[44mCyan on Blue (classic BBS style)\\033[0m"
echo -e "\\033[1;37m\\033[41mWhite on Red (error/warning)\\033[0m"
echo -e "\\033[1;33m\\033[40mYellow on Black (highlight)\\033[0m"
echo -e "\\033[0;32m\\033[40mGreen on Black (success)\\033[0m"

echo
echo "FULL COLOR MATRIX:"
echo "------------------"
echo "Text colors on different backgrounds:"

# Create a full matrix showing all foreground colors on all backgrounds
for bg in 0 1 2 3 4 5 6 7; do
    echo -n "BG$bg: "
    for fg in 0 1 2 3 4 5 6 7; do
        echo -ne "\\033[4${bg}m\\033[3${fg}m $fg \\033[0m"
    done
    echo -n " | "
    for fg in 0 1 2 3 4 5 6 7; do
        echo -ne "\\033[4${bg}m\\033[1;3${fg}m $fg \\033[0m"
    done
    echo
done

echo
echo "Legend: Left side = dark colors (|00-|07), Right side = bright colors (|08-|15)"
echo "=================================================="