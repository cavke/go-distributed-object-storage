### Start distributed storage with

``
make docker-run
``

### Put object
``
curl -X PUT -H "Content-Type: text/plain" --data "test file" http://localhost:3000/object/1
``

### Get object

``
curl http://localhost:3000/object/1
``