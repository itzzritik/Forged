default:
    @just --list

# Build
build-cli:
    cd cli && go build -o ../bin/forged ./cmd/forged
    cd cli && go build -o ../bin/forged-sign ./cmd/forged-sign
    ./scripts/build-forged-auth.sh

build-server:
    cd server && go build -o ../bin/forged-server ./cmd/forged-server

build-web:
    cd web && pnpm build

build: build-cli build-server

# Lint
lint-cli:
    cd cli && golangci-lint run ./...

lint-server:
    cd server && golangci-lint run ./...

lint-web:
    cd web && pnpm lint

lint: lint-cli lint-server

# Run
dev:
    just build-cli
    cd cli && go run ./cmd/forged-dev-service --binary ../bin/forged install

dev-stop:
    cd cli && go run ./cmd/forged-dev-service stop

dev-server:
    cd server && doppler run -- go run ./cmd/forged-server

dev-web:
    cd web && pnpm dev

auth:
    node -e '(async()=>{const fs=require("fs");const os=require("os");const cp=require("child_process");const http=require("http");const https=require("https");const path=require("path");const url=require("url");const web=process.env.FORGED_WEB_URL||"http://localhost:3035";const credsPath=path.join(os.homedir(),".forged","config","credentials.json");const parseJWTExpiry=(token)=>{try{const part=token.split(".")[1];if(!part)return NaN;const b64=part.replace(/-/g,"+").replace(/_/g,"/");const padded=b64.padEnd(Math.ceil(b64.length/4)*4,"=");const payload=JSON.parse(Buffer.from(padded,"base64").toString("utf8"));return typeof payload.exp==="number"?payload.exp*1000:NaN;}catch{return NaN;}};const openWith=(session)=>{const params=new url.URLSearchParams({access_token:session.access_token,access_expires_at:session.access_expires_at,refresh_token:session.refresh_token,refresh_expires_at:session.refresh_expires_at,user_id:session.user_id,email:session.email,name:session.name||""});cp.spawn("open",[web+"/api/auth/callback?"+params.toString()],{stdio:"inherit"});};if(!fs.existsSync(credsPath)){console.error("No local Forged credentials found. Log in first.");process.exit(1);}const creds=JSON.parse(fs.readFileSync(credsPath,"utf8"));const now=Date.now();const accessToken=(creds.access_token||creds.accessToken||creds.token||"").trim();const refreshToken=(creds.refresh_token||creds.refreshToken||"").trim();const serverURL=(creds.server_url||creds.serverURL||"").trim();const userID=(creds.user_id||creds.userID||"").trim();const email=(creds.email||"").trim();const name=(creds.name||"").trim();const accessExpiryRaw=creds.access_expires_at||creds.accessExpiresAt||"";const refreshExpiryRaw=creds.refresh_expires_at||creds.refreshExpiresAt||"";const accessExpiry=accessExpiryRaw?Date.parse(accessExpiryRaw):parseJWTExpiry(accessToken);const refreshExpiry=refreshExpiryRaw?Date.parse(refreshExpiryRaw):NaN;const accessExpiresAt=!Number.isNaN(accessExpiry)?new Date(accessExpiry).toISOString():"";const refreshExpiresAt=!Number.isNaN(refreshExpiry)?new Date(refreshExpiry).toISOString():"";if(!accessToken){console.error("No usable local Forged access token found. Log in first.");process.exit(1);}if(!refreshToken){if(Number.isNaN(accessExpiry)||accessExpiry<=now+60000){console.error("Legacy local credentials expired. Log in again in Forged, then rerun just auth.");process.exit(1);}openWith({access_token:accessToken,access_expires_at:accessExpiresAt,refresh_token:accessToken,refresh_expires_at:accessExpiresAt,user_id:userID,email,name});process.exit(0);}if(!serverURL){console.error("credentials.json is missing server_url.");process.exit(1);}const needsRefresh=Number.isNaN(accessExpiry)||accessExpiry<=now+60000;if(!needsRefresh){openWith({access_token:accessToken,access_expires_at:accessExpiresAt,refresh_token:refreshToken,refresh_expires_at:refreshExpiresAt||new Date(now+30*24*60*60*1000).toISOString(),user_id:userID,email,name});process.exit(0);}const body=JSON.stringify({refresh_token:refreshToken});const endpoint=new URL("/api/v1/auth/refresh",serverURL);const client=endpoint.protocol==="https:"?https:http;const req=client.request(endpoint,{method:"POST",headers:{"Content-Type":"application/json","Content-Length":Buffer.byteLength(body)}},res=>{let raw="";res.on("data",c=>raw+=c);res.on("end",()=>{if(res.statusCode!==200){console.error(raw||("Refresh failed: "+res.statusCode));process.exit(1);}const data=JSON.parse(raw);const next={server_url:serverURL,token:data.access_token,access_token:data.access_token,access_expires_at:data.access_expires_at,refresh_token:data.refresh_token,refresh_expires_at:data.refresh_expires_at,user_id:data.user_id||userID,email:data.email||email,name:data.name||name};fs.writeFileSync(credsPath,JSON.stringify(next,null,2));openWith({access_token:next.access_token,access_expires_at:next.access_expires_at,refresh_token:next.refresh_token,refresh_expires_at:next.refresh_expires_at,user_id:next.user_id,email:next.email,name:next.name});process.exit(0);});});req.on("error",err=>{console.error(err.message);process.exit(1);});req.write(body);req.end();})();'

# Database
migrate:
    cd server && doppler run -- go run ./cmd/migrate

migrate-reset:
    cd server && doppler run -- go run ./cmd/migrate reset

# Clean
clean:
    rm -rf bin
