## NSQ Analysis and Common Queries

### 1. Message Persistence in a Channel Without Consumers  
If no consumer is consuming messages from a channel, NSQ **does not persist messages indefinitely**. Messages remain in memory until:  
- A consumer connects and processes them.  
- The `--mem-queue-size` limit is reached, causing older messages to be discarded.  
- NSQ is restarted (as messages are not persisted across restarts unless `--data-path` is configured).  

By default, `--mem-queue-size` is **10000 messages**.

---

### 2. Changing the Message Retention Limit for a Channel  
Yes, the retention limit can be changed by configuring the `--mem-queue-size` parameter.  

```sh
nsqd --mem-queue-size=50000
```

This increases the in-memory queue to hold 50,000 messages if no consumer is connected. However, once the limit is reached, older messages are discarded.

---

### 3. Setting Up `nsqd` Locally on Windows  

#### **Step 1: Download NSQ**  
1. Go to [NSQ Releases](https://github.com/nsqio/nsq/releases).  
2. Download the Windows binary (`nsq-<version>.zip`).  
3. Extract it to a directory (e.g., `C:\nsq`).

#### **Step 2: Start `nsqd`**  
Open PowerShell and navigate to the extracted folder:  

```powershell
cd C:\nsq
.\nsqd.exe --lookupd-tcp-address=127.0.0.1:4160
```

#### **Step 3: Start `nsqlookupd`**  
In another PowerShell window, run:

```powershell
.\nsqlookupd.exe
```

#### **Step 4: Publish a Test Message**  
Use `nsq_pubsub` or `curl` to publish a message:

```powershell
curl -X POST http://127.0.0.1:4151/pub?topic=test_topic -d "Hello NSQ"
```

#### **Step 5: Start a Consumer**  
Run `nsq_tail` to consume messages:

```powershell
.\nsq_tail.exe --topic=test_topic --channel=test_channel --lookupd-http-address=127.0.0.1:4161
```

Now, messages published to `test_topic` will be received by the consumer.

---

### 4. Useful NSQ Utilities  

- **`nsq_stat`** → Provides real-time statistics of topics, channels, and messages.  
  ```powershell
  .\nsq_stat.exe --lookupd-http-address=127.0.0.1:4161
  ```
  
- **`nsq_admin`** → Web UI to monitor and manage NSQ nodes.  
  ```powershell
  .\nsq_admin.exe --lookupd-http-address=127.0.0.1:4161
  ```

- **`nsq_tail`** → Consumes messages in real-time from a topic/channel.
