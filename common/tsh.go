/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"os"
	//"path/filepath"
	//"bufio"
	"bytes"
	"io/ioutil"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	//"github.com/gravitational/teleport/lib/session"
	//"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/buger/goterm"
	"github.com/mvdan/sh/syntax"
)

// CLIConf stores command line arguments and flags:
type CLIConf struct {
	// Username is the Teleport user's username (to login into proxies)
	Username string
	// Proxy keeps the hostname:port of the SSH proxy to use
	Proxy string
	// TTL defines how long a session must be active (in minutes)
	MinsToLive int32
	// Login on a remote SSH host
	NodeLogin string
	// InsecureSkipVerify bypasses verification of HTTPS certificate when talking to web proxy
	InsecureSkipVerify bool
	// Interactive, when set to true, launches remote command with the terminal attached
	Interactive bool
	// Namespace is used to select cluster namespace
	Namespace string

	// Quiet mode, -q command (disables progress printing)
	Quiet bool
	// IsUnderTest is set to true for unit testing
	IsUnderTest bool

	// Number of parallel jobs to run
	MaxParallel int
	// Dry Run only
	DryRun bool
	// Delay between commands
	Delay int

	// Only one of the following selection methods is allowed
	// Select all hosts in the cluster
	SelectAll bool
	// Select a single host by name
	SelectSingle string
	// Provide a hostlist
	SelectList string
	// Provide a nodelist
	SelectNodelist string
	// Provide a groupfile
	SelectGroupfile string

	// Only one of the filter options can be selected
	// Glob filter
	FilterGlob string
	// Label filter
	FilterLabel string

	// Include options
	IncludeFile string
	IncludePath string
	SecretsFile string
	SecretsPath string

	ExtraVars []string

	// Unused options?
	// UserHost contains "[login]@hostname" argument to SSH command
	UserHost string
	// Commands to execute on a remote host
	RemoteCommand []string
	// SSH Port on a remote SSH host
	NodePort int16
	// LoadSystemAgentOnly when set to true will cause tsh agent to load keys into the system agent and
	// then exit. This is useful when calling tsh agent from a script (for example ~/.bash_profile)
	// to load keys into your system agent.
	LoadSystemAgentOnly bool
	// AgentSocketAddr is address for agent listeing socket
	AgentSocketAddr utils.NetAddrVal
	// Remote SSH session to join
	SessionID string
	// Src:dest parameter for SCP
	CopySpec []string
	// -r flag for scp
	RecursiveCopy bool
	// -L flag for ssh. Local port forwarding like 'ssh -L 80:remote.host:80 -L 443:remote.host:443'
	LocalForwardPorts []string
	// --local flag for ssh
	LocalExec bool
	// SiteName specifies remote site go login to
	SiteName string
}

// Run executes TSH client. same as main() but easier to test
func Run(args []string, underTest bool) {
	var (
		cf CLIConf
	)
	cf.IsUnderTest = underTest
	utils.InitLoggerCLI()

	app := utils.InitCLIParser("tpth", "Host automation tool built using Teleport").Interspersed(false)

	app.HelpFlag.Short('h')

	// Flags shared with other Teleport commands operation
	app.Flag("login", "Remote host login").Short('l').Envar("TELEPORT_LOGIN").StringVar(&cf.NodeLogin)
	localUser, _ := client.Username()
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", localUser)).Envar("TELEPORT_USER").StringVar(&cf.Username)
	app.Flag("cluster", "Specify the cluster to connect").Envar("TELEPORT_SITE").StringVar(&cf.SiteName)
	app.Flag("proxy", "SSH proxy host or IP address").Envar("TELEPORT_PROXY").StringVar(&cf.Proxy)
	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.MinsToLive)
	app.Flag("insecure", "Do not verify server's certificate and host name. Use only in test environments").Default("false").BoolVar(&cf.InsecureSkipVerify)
	app.Flag("namespace", "Namespace of the cluster").Default(defaults.Namespace).StringVar(&cf.Namespace)
	app.Flag("tty", "Allocate TTY").Short('t').BoolVar(&cf.Interactive)

	// Flags just for managing parallel execution
	app.Flag("parallel", "Maximum number of parallel nodes [default=numcpu]").Short('p').IntVar(&cf.MaxParallel)
	app.Flag("dryrun", "Dry-run. Output the command template results, but don't run remotely").BoolVar(&cf.DryRun)
	app.Flag("delay", "Delay between remote command execution").Default("1").IntVar(&cf.Delay)

	// Host Selection Flags
	app.Flag("all", "Select all hosts in cluster [default]").Short('A').BoolVar(&cf.SelectAll)
	app.Flag("onehost", "Select one host(node)").Short('o').StringVar(&cf.SelectSingle)
	app.Flag("hostlist", "Select hosts with a comma separated list of hostnames").Short('H').StringVar(&cf.SelectList)
	app.Flag("nodelist", "Select hosts with a comma separated list of nodeids").Short('N').StringVar(&cf.SelectNodelist)
	app.Flag("groupfile", "Select hosts with a path to a group file, one hostname per line ").Short('G').StringVar(&cf.SelectGroupfile)

	// Host filter rules
	app.Flag("glob", "Filter selected hosts with hostname glob").Short('g').StringVar(&cf.FilterGlob)
	app.Flag("label", "Filter selected hosts with label=value match").Short('L').StringVar(&cf.FilterLabel)

	// selection is ( cluster & namespace ) -> ( A | h | H | N | G ) -> ( g | L )

	// Include options
	app.Flag("include", "Include a single file template").Short('i').StringVar(&cf.IncludeFile)
	app.Flag("incpath", "Include a directory of templates").Short('I').StringVar(&cf.IncludePath)
	app.Flag("secrets", "Include a single secrets file").Short('s').StringVar(&cf.SecretsFile)
	app.Flag("secpath", "Include a directory of secrets files").Short('S').StringVar(&cf.SecretsPath)

	debugMode := app.Flag("debug", "Verbose logging to stdout").Short('d').Bool()
	app.Flag("quiet", "Quiet mode. Reduce output").Short('q').BoolVar(&cf.Quiet)
	versionMode := app.Flag("version", "Show the version of tpth").Short('v').Bool()

	// Final arguments
	template := app.Arg("template", "Path to template file for remote execution").Required().String()
	app.Arg("key=value", "Additional key-value pairs to pass in the default pipeline as \".\"").StringsVar(&cf.ExtraVars)

	// parse CLI commands+flags:
	_, err := app.Parse(args)
	if err != nil {
		utils.FatalError(err)
	}

	// apply -d flag:
	if *debugMode {
		utils.InitLoggerDebug()
	}

	if *versionMode {
		onVersion()
		os.Exit(0)
	}

	parseTemplate(*template, &cf)

}

// parse ExtraVars into map[string]string
func parseExtraVars(cf *CLIConf) (vars map[string]string) {

	vars = make(map[string]string)

	for _, v := range cf.ExtraVars {

		s := strings.SplitN(v, "=", 2)
		key := s[0]
		value := s[1]

		vars[key] = value

	}

	return vars
}

// parseTemplate returns the parsed template
func parseTemplate(tmplfile string, cf *CLIConf) {

	var err error

	//defer func() {
	//	if recover() != nil {
	//		fmt.Println("An unhandled exception was caught.")
	//	}
	//}()

	// The buffer holding the processed template
	buf := &bytes.Buffer{}

	// Initialize the main template
	tmpl := template.New("__main__")

	// Process the supplemental key=value pairs
	v := parseExtraVars(cf)

	// Process functions
	funcs := template.FuncMap{

		"listTemplates": func() string {
			var templateList string
			for _, r := range tmpl.Templates() {
				templateList = templateList + r.Name() + "\n"
			}
			return templateList
		},
	}
	tmpl.Funcs(funcs)

	var incFile []byte

	// Read the single include file (if it exists)
	if cf.IncludeFile != "" {
		incFile, err = ioutil.ReadFile(cf.IncludeFile)
		if err != nil {
			fmt.Println("# Aborting due incfile read error:", err)
			os.Exit(1)
		}

		if incFile[len(incFile)-1] != '\n' {
			incFile = append(incFile, '\n')
		}
	}

	// Read the main template
	mainFile, err := ioutil.ReadFile(tmplfile)
	if err != nil {
		fmt.Println("# Aborting due file read error:", err)
		os.Exit(1)
	}

	_, err = tmpl.Parse(string(incFile) + string(mainFile))
	if err != nil {
		fmt.Println("# Error parsing main file:", err)
		os.Exit(1)
	}

	// Add the IncludePath
	if cf.IncludePath != "" {
		_, err = tmpl.ParseFiles(cf.IncludePath)
		if err != nil {
			fmt.Println("# error parsing include path files:", err)
			os.Exit(1)
		}
	}

	err = tmpl.ExecuteTemplate(buf, "__main__", v)
	if err != nil {
		fmt.Println("# Aborting due to execute error:", err)
		os.Exit(1)
	}

	sh, err := syntax.Parse(buf, "", syntax.ParseComments)
	if err != nil {
		//fmt.Println(buf)
		fmt.Println("# Aborting due to shell parse error:", err)

		os.Exit(1)
	}
	var out syntax.PrintConfig
	out.Spaces = 2
	out.Fprint(os.Stdout, sh)

	// scanner := bufio.NewScanner(buf)
	//	for scanner.Scan() {
	//		goterm.Print(scanner.Text())
	//		goterm.Flush()
	//		time.Sleep(time.Duration(cf.Delay) * time.Second)
	//	}
	//	if err := scanner.Err(); err != nil {
	//		fmt.Println("# Aborting due to execution error:", err)
	//	}

	// fmt.Print(buf)
}

// onListNodes executes 'tsh ls' command
func onListNodes(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	servers, err := tc.ListNodes(context.TODO())
	if err != nil {
		utils.FatalError(err)
	}
	nodesView := func(nodes []services.Server) string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Node Name", "Node ID", "Address", "Labels"})
		if len(nodes) == 0 {
			return t.String()
		}
		for _, n := range nodes {
			fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", n.GetHostname(), n.GetName(), n.GetAddr(), n.LabelsString())
		}
		return t.String()
	}
	fmt.Printf(nodesView(servers))
}

// onListSites executes 'tsh sites' command
func onListSites(cf *CLIConf) {
	tc, err := makeClient(cf, true)
	if err != nil {
		utils.FatalError(err)
	}
	proxyClient, err := tc.ConnectToProxy()
	if err != nil {
		utils.FatalError(err)
	}
	defer proxyClient.Close()

	sites, err := proxyClient.GetSites()
	if err != nil {
		utils.FatalError(err)
	}
	sitesView := func() string {
		t := goterm.NewTable(0, 10, 5, ' ', 0)
		printHeader(t, []string{"Cluster Name", "Status"})
		if len(sites) == 0 {
			return t.String()
		}
		for _, site := range sites {
			fmt.Fprintf(t, "%v\t%v\n", site.Name, site.Status)
		}
		return t.String()
	}
	quietSitesView := func() string {
		names := make([]string, 0)
		for _, site := range sites {
			names = append(names, site.Name)
		}
		return strings.Join(names, "\n")
	}
	if cf.Quiet {
		sitesView = quietSitesView
	}
	fmt.Printf(sitesView())
}

// onSSH executes 'tsh ssh' command
func onSSH(cf *CLIConf) {
	tc, err := makeClient(cf, false)
	if err != nil {
		utils.FatalError(err)
	}

	tc.Stdin = os.Stdin
	if err = tc.SSH(context.TODO(), cf.RemoteCommand, cf.LocalExec); err != nil {
		// exit with the same exit status as the failed command:
		if tc.ExitStatus != 0 {
			fmt.Fprintln(os.Stderr, utils.UserMessageFromError(err))
			os.Exit(tc.ExitStatus)
		} else {
			utils.FatalError(err)
		}
	}
}

// makeClient takes the command-line configuration and constructs & returns
// a fully configured TeleportClient object
func makeClient(cf *CLIConf, useProfileLogin bool) (tc *client.TeleportClient, err error) {
	// apply defults
	if cf.MinsToLive == 0 {
		cf.MinsToLive = int32(defaults.CertDuration / time.Minute)
	}

	// split login & host
	hostLogin := cf.NodeLogin
	var labels map[string]string
	if cf.UserHost != "" {
		parts := strings.Split(cf.UserHost, "@")
		if len(parts) > 1 {
			hostLogin = parts[0]
			cf.UserHost = parts[1]
		}
		// see if remote host is specified as a set of labels
		if strings.Contains(cf.UserHost, "=") {
			labels, err = client.ParseLabelSpec(cf.UserHost)
			if err != nil {
				return nil, err
			}
		}
	}
	fPorts, err := client.ParsePortForwardSpec(cf.LocalForwardPorts)
	if err != nil {
		return nil, err
	}

	// 1: start with the defaults
	c := client.MakeDefaultConfig()

	// 2: override with `./tsh` profiles (but only if no proxy is given via the CLI)
	if cf.Proxy == "" {
		if err = c.LoadProfile(""); err != nil {
			fmt.Printf("WARNING: Failed loading tsh profile.\n%v\n", err)
		}
	}

	// 3: override with the CLI flags
	if cf.Namespace != "" {
		c.Namespace = cf.Namespace
	}
	if cf.Username != "" {
		c.Username = cf.Username
	}
	if cf.Proxy != "" {
		c.ProxyHostPort = cf.Proxy
	}
	if len(fPorts) > 0 {
		c.LocalForwardPorts = fPorts
	}
	if cf.SiteName != "" {
		c.SiteName = cf.SiteName
	}
	// if host logins stored in profiles must be ignored...
	if !useProfileLogin {
		c.HostLogin = ""
	}
	if hostLogin != "" {
		c.HostLogin = hostLogin
	}
	c.Host = cf.UserHost
	c.HostPort = int(cf.NodePort)
	c.Labels = labels
	c.KeyTTL = time.Minute * time.Duration(cf.MinsToLive)
	c.InsecureSkipVerify = cf.InsecureSkipVerify
	c.Interactive = cf.Interactive
	return client.NewClient(c)
}

func onVersion() {
	utils.PrintVersion()
}

func printHeader(t *goterm.Table, cols []string) {
	dots := make([]string, len(cols))
	for i := range dots {
		dots[i] = strings.Repeat("-", len(cols[i]))
	}
	fmt.Fprint(t, strings.Join(cols, "\t")+"\n")
	fmt.Fprint(t, strings.Join(dots, "\t")+"\n")
}

// refuseArgs helper makes sure that 'args' (list of CLI arguments)
// does not contain anything other than command
func refuseArgs(command string, args []string) {
	if len(args) == 0 {
		return
	}
	lastArg := args[len(args)-1]
	if lastArg != command {
		utils.FatalError(trace.BadParameter("%s does not expect arguments", command))
	}
}
