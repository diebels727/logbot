package logbot

import(
  // "fmt"
  "github.com/diebels727/go-ircevent"
  "flag"
  "time"
  "fmt"
  "os"
  "path"
  "path/filepath"
  "strings"
  // "io/ioutil"
  // "strconv"
  // "syscall"
  "github.com/nightlyone/lockfile"
  "code.google.com/p/go-sqlite/go1/sqlite3"
)

type LogBot struct {
  server string
  channel string
  channels []string
  port string
  botname string
  username string
}

func init() {
  flag.StringVar(&server,"server","irc.freenode.org","IRC server FQDN")
  flag.StringVar(&channel,"channel","#cinch-bots","IRC channel name (including #-sign)")
  flag.StringVar(&port,"port","6667","IRC server port number")
  flag.StringVar(&botname,"botname","logbot","Name of the bot visible on IRC channel")
  flag.StringVar(&username,"username","logbot","Username to login with to IRC")
}

func path_exists(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil { return true, nil }
    if os.IsNotExist(err) { return false, nil }
    return false, err
}


func check(e error) {
    if e != nil {
        panic(e)
    }
}

func main() {
  flag.Parse()
  // log := make(chan string)
  sql := make(chan string)  //this will be buffered

  //replace all instances of .'s
  server_slug := strings.Replace(server,".","-",-1)
  //remove #'s and .'s from channel
  channel_slug := strings.Replace(channel,"#","",-1)
  channel_slug = strings.Replace(channel_slug,".","-",-1)

  //lockfile so we do no harm
  log_path := path.Join(server_slug,channel_slug)
  abs_log_path,err := filepath.Abs(log_path)
  check(err)
  file_mode := os.FileMode(0777)
  err = os.MkdirAll(log_path,file_mode)
  check(err)
  lock_file,err := lockfile.New(path.Join(abs_log_path,"lock"))
  check(err)
  err = lock_file.TryLock()
  check(err)

  //persistence
  db_file := "db.sqlite3"
  db_path := path.Join(server_slug,channel_slug,db_file)
  db, err := sqlite3.Open(db_path)
  check(err)
  defer db.Close()

  db.Exec(`CREATE TABLE events(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER,
    username VARCHAR(64),
    host VARCHAR(255),
    message TEXT
  );`)

  con := irc.IRC(username,botname)
  con.Debug = true
  con.VerboseCallbackHandler = true
  err = con.Connect(server + ":" + port)
  check(err)

  con.AddCallback("001", func (e *irc.Event) {
    byte_channel := []byte(channel)
    //prepend hash if not hashed
    if !(byte_channel[0] == []byte("#")[0]) {
      channel = "#"+channel
    }
    con.Join(channel)
    // con.SendRaw("LIST")
  })
  con.AddCallback("PRIVMSG", func (e *irc.Event) {
    // t := time.Now().UTC()
    // t_integer := int64(t)
    // db.Exec(`INSERT INTO events(message) VALUES("first message")`)
    // timestamp := t.Format("20060102150405")
    // message := "{" + " \\\"timestamp\\\": " + "\\\""+timestamp+"\\\"," + "\\\"username\\\": "+"\\\""+e.Nick+"\\\"," + "\\\"host\\\": " + "\\\"" + e.Host + "\\\"," + "\\\"message\\\": " + "\\\"" + e.Message() + "\\\""+"}"
    // log <- message
    t := time.Now().Unix()
    t_str := fmt.Sprintf("%d",t)
    sql_statement := "INSERT INTO events(timestamp,username,host,message) VALUES("+"\""+t_str+"\""+","+"\""+e.Nick+"\""+","+"\""+e.Host+"\""+","+"\""+e.Message()+"\""+");"
    sql <- sql_statement
  })

  //this is an event for receiving a channel from a channel list
  //each channel listed triggers an event
  con.AddCallback("322",func(e *irc.Event) {
    // channel_event := e.Arguments[1]
    // channels = append(channels,channel_event)
  })

  //this event is triggered when the channel list is done (and perhaps when it begins)
  con.AddCallback("323",func(e *irc.Event) {
    // for _,v := range channels {
    //   fmt.Println("Channel: ",v)
    // }
    con.Join(channel)
  })

  // go func(log chan string) {
  //   for {
  //     message := <- log
  //     fmt.Println(message)
  //   }
  // }(log)

  go func(sql chan string) {
    for {

      statement := <- sql

      // fmt.Println("SQL: ",statement)

      db.Exec(statement)
    }
  }(sql)

  con.Loop()
}
