#!/usr/bin/env sh
toml="./cfg/config.toml"
env="./deployments/.env"

extract() {
  grep -E "^$1" "$toml" | sed -E 's/.*=[[:space:]]*"?(.*?)"?$/\1/' | tr -d '"'
}

user=$(extract "User")
pass=$(extract "Password")
db=$(extract "Database")

cat > "$env" << EOF
POSTGRES_USER=$user
POSTGRES_PASSWORD=$pass
POSTGRES_DB=$db
EOF

echo "created .env"