# telepath

Host automation tool built on top of Teleport.

```
Usage: tpth [<flags>] <path/to/template/script> [<key>=<value> ...]

Telepathy will execute a script on a remote host thhrough Teleport using
a template script.

 
 Standard Options

  -l, --login      Remote host login
      --user       SSH proxy user [local username]
      --cluster    Specify the cluster to connect
      --proxy      SSH proxy host or IP address
      --insecure   Do not verify server's certificate and host name. Use only in test environments
      --namespace  Namespace of the cluster
  -d, --debug      Verbose logging to stdout

  Host Selection

  -A, --all        Select all nodes in the cluster and namespace (default)
  -h, --host       Single host(node) selection
  -H, --hostlist   Host(node) list
                   Comma(",") delimited list of hosts(nodes)
  -N, --nodeids    Node IDs
                   Comma(",") dlimited list of node ids
  -G, --groupfile  File for group host(node) actions
                   One hostname(nodename) per line
  -g, --glob       Hostname glob for group host(node) actions
                   Applies either to the whole Teleport node list in the cluster or to the group file
  -L, --labels     Match of host(node) labels
  -p, --parallel   Number of hosts(nodes) to run in parallel [numcpu-1]

  The order of host selection is ( cluster & namespace ) -> ( A | h | H | N | G ) -> ( g | L )

  Template Include

  -i, --include    Additional file to include in the template processing
                   Supports %c, %h, and %u expansions for cluster, host(node), and user
  -I, --incpath    Include path to include in template process
                   Colon (":") delimited list of directories
  -s, --secrets    Secrets file to include in the template processing
                   File is a ASCII-armored gpg-encrypted template file
                   Supports %c, %h, and %u expansions for cluster, host(node), and user
  -S, --secpath    Include path to include additional secrets processing
                   Colon(":") delimited list of directories

  Other
      --dryrun     Dry run
                   Show node selection and expand parse template for each node, but don't actually run remote commands.
      --delay      Delay in seconds between commands [default=1]

```

Template processing notes

  key=value options are passed as additional variables to the template.
  labels are also passed as additional variables to the template


