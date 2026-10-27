import base64, re, sys, pathlib

dom = sys.stdin.read()
out = pathlib.Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
for name, url in re.findall(r'<pre data-name="([^"]+)">data:[^;]+;base64,([^<]+)</pre>', dom):
    data = base64.b64decode(url)
    (out / name).write_bytes(data)
    print(name, len(data))
