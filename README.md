```bash
cat > README.md << 'EOF'
# farum

A minimal pseudo-container runtime built from scratch in Go.

## Usage (for now...)

```bash
farum pull ubuntu:22.04
farum run ubuntu:22.04 /bin/bash
```
EOF

git add README.md
git commit -m "add readme"
git push -u origin main --force
```