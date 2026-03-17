package builtins

import (
	"github.com/grimdork/climate/cfmt"
)

// help topics keyed by command name. Each entry is a cfmt-formatted string.
var helpTopics = map[string]string{
	"cd": `%bold%lwhite cd%reset %cyan[dir]%reset

  Change the current working directory.

  If no directory is given, changes to %bold$HOME%reset.
  Supports %bold~%reset as shorthand for the home directory.

  %grey Examples:%reset
    %cyancd%reset /tmp
    %cyancd%reset ~
    %cyancd%reset`,

	"history": `%bold%lwhite history%reset

  Display the command history from %bold~/.kush_history%reset.`,

	"alias": `%bold%lwhite alias%reset %cyan[name='value' | -r]%reset

  Without arguments, lists all defined aliases in sorted order.
  With %bold%cr%reset, reloads aliases from disk into the in-memory cache.
  With %cyanname='value'%reset, defines or replaces an alias.

  Aliases are stored in %bold~/.kush_aliases%reset (or %bold$KUSH_ALIASES%reset).

  %grey Examples:%reset
    %cyanalias%reset                         List all aliases
    %cyanalias%reset %cyan-r%reset                      Reload from disk
    %cyanalias%reset %cyanll='ls -la'%reset             Define an alias
    %cyanalias%reset %cyngrep='grep --color'%reset     Colourised grep`,

	"unalias": `%bold%lwhite unalias%reset %cyanname%reset

  Remove an alias by name and save the updated file.

  %grey Example:%reset
    %cyanunalias%reset %cyanll%reset`,

	"reload": `%bold%lwhite reload%reset

  Reload aliases from disk into the in-memory cache.
  Equivalent to %cyanalias%reset %cyan-r%reset.
  Can also be triggered externally with %cyankill%reset %cyan-HUP <pid>%reset.`,

	"which": `%bold%lwhite which%reset %cyanprog [prog...]%reset

  Show the full path for one or more programs by searching %bold$PATH%reset.

  %grey Example:%reset
    %cyanwhich%reset %cyango%reset %cyngcc%reset %cynmake%reset`,

	"checksum": `%bold%lwhite checksum%reset %cyan[md5|sha1|sha256] file%reset

  Compute a checksum of the given file.
  %grey(Not yet implemented.)%reset`,

	"help": `%bold%lwhite help%reset %cyan[command]%reset

  Without arguments, shows a summary of all builtins.
  With a command name, shows detailed help for that command.

  %grey Example:%reset
    %cyanhelp%reset %cyanalias%reset`,

	"exit": `%bold%lwhite exit%reset / %cyanCtrl+D%reset

  Exit the shell.`,
}

// printHelp handles the "help" builtin. Returns true if it was handled.
func printHelp(args []string) {
	if len(args) > 1 {
		topic := args[1]
		if text, ok := helpTopics[topic]; ok {
			cfmt.Println("")
			cfmt.Println(text)
			cfmt.Println("")
			return
		}
		cfmt.Printf("%%yellowNo help available for %%bold%s%%reset%%yellow.%%reset\n", topic)
		return
	}

	// General help overview.
	cfmt.Println("")
	cfmt.Println("%bold%lwhitekush%reset — a tiny custom shell")
	cfmt.Println("")
	cfmt.Println("%bold  Builtins:%reset")
	cfmt.Println("    %cyan cd%reset [dir]                  Change directory")
	cfmt.Println("    %cyan history%reset                   Show command history")
	cfmt.Println("    %cyan alias%reset [name='val' | -r]   List, define, or reload aliases")
	cfmt.Println("    %cyan unalias%reset name              Remove an alias")
	cfmt.Println("    %cyan reload%reset                    Reload aliases from disk")
	cfmt.Println("    %cyan which%reset prog [...]          Show program path")
	cfmt.Println("    %cyan checksum%reset algo file        Compute file checksum")
	cfmt.Println("    %cyan help%reset [command]            This help")
	cfmt.Println("")
	cfmt.Println("%bold  Key bindings:%reset")
	cfmt.Println("    %cyan Ctrl+D%reset      Exit")
	cfmt.Println("    %cyan Ctrl+C%reset      Clear line")
	cfmt.Println("    %cyan Ctrl+W%reset      Delete word left")
	cfmt.Println("    %cyan Ctrl+U%reset      Kill to start of line")
	cfmt.Println("    %cyan Ctrl+K%reset      Kill to end of line")
	cfmt.Println("    %cyan Alt+←/→%reset     Move by word")
	cfmt.Println("")
	cfmt.Println("  Type %bold%cyanhelp%reset %boldcommand%reset for details on a specific builtin.")
	cfmt.Println("")
}
