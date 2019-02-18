This package is a derivative of https://github.com/pkg/sftp

It was modified to strip the purely functional protocol specific aspects of the protocol out,
which let me focus on fixing other issues I had with that package, such as out of order
packet processing.