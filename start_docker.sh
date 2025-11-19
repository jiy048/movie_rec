## if you want to override dockerfile



docker run -it --rm -p 8080:8080 -v "$PWD":/app -w /app go-app:0.1 sh 
