wget -nc https://github.com/hairyhenderson/gomplate/releases/latest/download/gomplate_linux-amd64 -O gomplate
chmod +x gomplate
./gomplate -d config=cfg/config.toml -f .env.tmpl -o .env

docker compose up -d --build

