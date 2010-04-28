#!/usr/bin/env python3
import struct
import io

def bitflip(x, n):
    r = 0
    while n:
        r = (r << 1) | (x & 1)
        x >>= 1
        n -= 1
    return r

table1 = [(2 << i) - 1 for i in range(16)]

#table2 = [
#    [0x01, 0x32, 0x76, 0x45, 0xfe, 0xcd, 0x89, 0xba],
#    [0xfe, 0xcd, 0x89, 0xba, 0x01, 0x32, 0x76, 0x45],
#    [0x08, 0xc4, 0xe6, 0x2a, 0xf7, 0x3b, 0x19, 0xd5],
#    [0xf7, 0x3b, 0x19, 0xd5, 0x08, 0xc4, 0xe6, 0x2a],
#]
table2 = [
    [0, 1, 3, 2, 7, 6, 4, 5, 0xf, 0xe, 0xc, 0xd, 8, 9, 0xb, 0xa],
    [0xf, 0xe, 0xc, 0xd, 8, 9, 0xb, 0xa, 0, 1, 3, 2, 7, 6, 4, 5],
]
table2_mirrored = [
    [bitflip(x, 4) for x in table2[0]],
    [bitflip(x, 4) for x in table2[1]],
]

#0 1 3 2 7 6 4 5
#f e c d 8 9 b a

#0 1 3 2
#7 6 4 5

#0 1
#3 2

#0 1 3 2
#7 6 4 5
#f e c d
#8 9 b a

#f e c d
#8 9 b a
#0 1 3 2
#7 6 4 5


table3 = [bitflip(i, 4) for i in range(16)]

def decompress(f):
    global sizex, sizey, mode2
    ram = []

    bs = bitstream(f)

    sizex = readint(bs, 4) * 8
    sizey = readint(bs, 4)
    ramorder = next(bs)

    ram1 = []
    ram2 = []
    rams = [ram1, ram2]
    fillram(rams[ramorder], bs)
    mode2 = next(bs)
    if mode2 == 1:
        mode2 = 1 + next(bs)
    fillram(rams[ramorder ^ 1], bs)

    ram1 = bitgroups_to_bytes(ram1)
    ram1 = bytearray(ram1)
    ram2 = bitgroups_to_bytes(ram2)
    ram2 = bytearray(ram2)

    if mode2 == 0:
        thing1(ram1)
        thing1(ram2)
    elif mode2 == 1:
        #thing1(ram1)
        thing2(ram1, ram2)
    elif mode2 == 2:
        thing1(rams[ramorder^1])
        thing2(rams[ramorder], rams[ramorder ^ 1])

    out = bytearray()
    for a, b in zip(ram1, ram2):
        out.append(a)
        out.append(b)
    return bytes(out)

def fillram(ram, bs):
    size = sizex*sizey * 4
    mode = ['rle', 'data'][next(bs)]
    while len(ram) < size:
        if mode == 'rle':
            rle(ram, bs)
            mode = 'data'
        elif mode == 'data':
            data(ram, bs)
            mode = 'rle'
        else:
            assert False
        #print(hex(len(ram)))
        if len(ram) > size:
            raise ValueError(size, len(ram))

def bitgroups_to_bytes(bits):
    l = []
    for i in range(0, len(bits)-3, 4):
        n = ((bits[i] << 6)
             | (bits[i+1] << 4)
             | (bits[i+2] << 2)
             | (bits[i+3]))
        l.append(n)
    return bytes(l)

def bitstream(f):
    while 1:
        char = f.read(1)
        if not char:
            break
        byte = char[0]

        yield (byte >> 7) & 1
        yield (byte >> 6) & 1
        yield (byte >> 5) & 1
        yield (byte >> 4) & 1
        yield (byte >> 3) & 1
        yield (byte >> 2) & 1
        yield (byte >> 1) & 1
        yield byte & 1

def readint(bs, count):
    n = 0
    while count:
        n <<= 1
        n |= next(bs)
        count -= 1
    return n

# the rle segment encodes chunks of zeros
def rle(ram, bs):
    # count bits until we find a 0
    i = 0
    while next(bs):
        i += 1
    
    n = table1[i]
    a = readint(bs, i+1)
    n += a

    #print(i, a, n)

    for i in range(n):
        ram.append(0)

# data encodes pairs of bits
def data(ram, bs):
    while 1:
        bitgroup = readint(bs, 2)
        # if we encounter a pair of 0 bits, we're done
        if bitgroup == 0:
            break
        ram.append(bitgroup)
        #print("d: {:02b}".format(bitgroup))

def thing1(ram, mirror=False):
    for x in range(sizex):
        prev = 0
        for y in range(sizey):
            i = y*sizex + x
            a = ram[i] >> 4
            b = ram[i] & 0xf
            table = table2 if not mirror else table2_mirrored

            bit = bool(prev & 1 if not mirror else prev & 8)
            a = table[bit][a]
            prev = a

            bit = bool(prev & 1 if not mirror else prev & 8)
            b = table[bit][b]
            prev = b

            ram[i] = (a << 4) | b

def thing2(ram1, ram2, mirror=False):
    thing1(ram1)

    for y in range(sizey):
        for x in range(sizex):
            i = y*sizex + x
            if mirror:
                #XXX
                pass
            ram1[i] ^= ram2[i]

f = open("../../red.gb", 'rb')
f.seek(0x34000)
out = decompress(f)
from binascii import hexlify
#for i in range(sizey):
    #for j in range(4):
        #row = b''.join(out[i*sizex + j*4 + k*16:i*sizex + j*4 + k*16 + 4] for k in range(4))
        #print(hexlify(row))
sizex //= 8
for i in range(sizey*4):
    row = out[i*sizex*4:(i+1)*sizex*4]
    print(hexlify(row))

