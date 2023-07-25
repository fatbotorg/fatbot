# SSHing into FLY

### Create a key
```bash
flyctl ssh issue --agent
```

### In a separate terminal:
```bash
fly proxy 10022:22
```
### In the main terminal:
```bash
scp -P 10022 root@localhost:/remote_path/to_file local_path/to_file 
```
