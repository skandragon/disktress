# disktress

Disktress is a simple hard drive tester.

Currently, it will write a known, reproducable pattern to a file (or raw disk
device), read it back, and verify that it read what it wrote.

# Strategy

For each block, a pattern generated based on the input `seed` and the `block` number
is computed, and used to generate a stream of bytes to fill the block.  Block sizes
can be of any length that is a multiple of 64.  Currently, blake2b is used to generate
the byte stream.

Data can be written only, verified only, or written then verified.

I commonly use a single iteration of write+verify, and then run verify only until
i'm satisfied the drive consistently returns correct data. I've also used a
write+verify loop, with a changing seed.

If an error is found, an exit status other than 0 will occur.
