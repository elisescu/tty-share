.PHONY: all frontend clean cleanfront rebuild

all: frontend
	@echo "Done"

rebuild: clean all

frontend: cleanfront frontend/public/index.html

frontend/public/index.html:
	cd frontend && npm install && npm run build && cd -

cleanfront:
	rm -fr frontend/public

clean: cleanfront
