#!/usr/bin/env python3
import struct
import io
import sys

def bitflip(x, n):
    r = 0
    while n:
        r = (r << 1) | (x & 1)
        x >>= 1
        n -= 1
    return r

class Decompressor:
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

    table3 = [bitflip(i, 4) for i in range(16)]

    tilesize = 8

    def __init__(self, f):
        self.bs = fbitstream(f)

        self.sizex = readint(self.bs, 4) * self.tilesize
        self.sizey = readint(self.bs, 4)

        self.size = self.sizex * self.sizey

        self.ramorder = next(self.bs)

        self.mirror = False

    def decompress(self):
        rams = [[], []]

        r1 = self.ramorder
        r2 = self.ramorder ^ 1

        self.fillram(rams[r1])
        mode = self.readbit()
        if mode == 1:
            mode = 1 + self.readbit()
        self.fillram(rams[r2])

        rams[0] = bytearray(bitgroups_to_bytes(rams[0]))
        rams[1] = bytearray(bitgroups_to_bytes(rams[1]))

        if mode == 0:
            self.thing1(rams[0])
            self.thing1(rams[1])
        elif mode == 1:
            self.thing1(rams[r1])
            self.thing2(rams[r1], rams[r2])
        elif mode == 2:
            self.thing1(rams[r2], mirror=False)
            self.thing1(rams[r1])
            self.thing2(rams[r1], rams[r2])

        out = []
        for a, b in zip(bitstream(rams[0]), bitstream(rams[1])):
            out.append(a | (b << 1))
            #out.append((a << 1) | b)
        return bitgroups_to_bytes(out)

    def fillram(self, ram):
        mode = ['rle', 'data'][self.readbit()]
        size = self.size * 4
        while len(ram) < size:
            if mode == 'rle':
                self.rle(ram)
                mode = 'data'
            elif mode == 'data':
                self.data(ram)
                mode = 'rle'
            else:
                assert False

            if len(ram) > size:
                raise ValueError(size, len(ram))
        ram[:] = self._deinterlace_bitgroups(ram)

    # the rle segment encodes chunks of zeros
    def rle(self, ram):
        # count bits until we find a 0
        i = 0
        while self.readbit():
            i += 1

        n = self.table1[i]
        a = self.readint(i+1)
        n += a

        #print(i, a, n)

        for i in range(n):
            ram.append(0)

    # data encodes pairs of bits
    def data(self, ram):
        while 1:
            bitgroup = self.readint(2)
            # if we encounter a pair of 0 bits, we're done
            if bitgroup == 0:
                break
            ram.append(bitgroup)

    def thing1(self, ram, mirror=None):
        if mirror is None:
            mirror = self.mirror

        table = self.table2 if not mirror else self.table2_mirrored
        for x in range(self.sizex):
            prev = 0
            for y in range(self.sizey):
                i = y*self.sizex + x
                a = ram[i] >> 4
                b = ram[i] & 0xf

                bit = bool(prev & 1 if not mirror else prev & 8)
                a = table[bit][a]
                prev = a

                bit = bool(prev & 1 if not mirror else prev & 8)
                b = table[bit][b]
                prev = b

                ram[i] = (a << 4) | b

    def thing2(self, ram1, ram2, mirror=None):
        if mirror is None:
            mirror = self.mirror

        for i in range(len(ram2)):
            if mirror:
                a = ram2[i] >> 4
                b = ram2[i] & 0xf
                a = self.table3[a]
                b = self.table3[b]
                ram2[i] = a << 4 | b

            ram2[i] ^= ram1[i]

    def _deinterlace_bitgroups(self, bits):
        l = []
        for y in range(self.sizey):
            for x in range(self.sizex):
                i = 4 * y * self.sizex + x
                for j in range(4):
                    l.append(bits[i])
                    i += self.sizex
        return l

    def readbit(self):
        return next(self.bs)

    def readint(self, n):
        return readint(self.bs, n)


def fbitstream(f):
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

def bitstream(b):
    for byte in b:
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

def bitgroups_to_bytes(bits):
    l = []
    for i in range(0, len(bits)-3, 4):
        n = ((bits[i] << 6)
             | (bits[i+1] << 4)
             | (bits[i+2] << 2)
             | (bits[i+3]))
        l.append(n)
    return bytes(l)

# PIL is not yet available for python 3, so we'll write out a pgm(5) file,
# and let netpbm(1) sort it out.
def savepam(ram, out):
    print("P7", file=out)
    print("HEIGHT", sizey*4, file=out)
    print("WIDTH", sizex//2, file=out)
    print("MAXVAL", 3, file=out)
    print("DEPTH", 1, file=out)
    print("TUPLETYPE", "GRAYSCALE", file=out)
    print("ENDHDR", file=out)
    i = 0
    for _ in range(sizey*4):
        for _ in range(sizex//2):
            byte = ram[i]
            print(byte>>6, byte>>4 & 3, byte >> 2 & 3, byte & 3, end=" ", file=out)
            i += 1

def savepgm(dcmp, ram, out):
    print("P2", file=out)
    print(dcmp.sizex, dcmp.sizey*8, file=out) # width, height
    print(3, file=out) # maxval
    i = 0
    width = dcmp.sizex // 4
    for i in range(len(ram)):
        byte = ram[i]
        #print(byte>>6, byte>>4 & 3, byte >> 2 & 3, byte & 3, end=" ", file=out)
        print(3 - (byte>>6), 3 - (byte>>4 & 3), 3 - (byte >> 2 & 3), 3 - (byte & 3), end=" ", file=out)
        #print(3 - (byte & 3), 3 - (byte>>2 & 3), 3 - (byte >> 4 & 3), 3 - (byte >> 6), end=" ", file=out)
        i += 1
        if i % width == 0:
            print(file=out)


def untile(self, ram):
    out = []
    for y in range(self.sizey*8):
        for x in range(self.sizex//8):
            k = (y + self.sizey * 8 * x) * 2
            out.append(ram[k:k+2])
    return b''.join(out)

def untile_mirror(ram):
    out = []
    for y in range(sizey*8):
        for x in reversed(range(sizex//8)):
            k = (y + sizey * 8 * x) * 2
            out.append(ram[k:k+2])
    return b''.join(out)

f = open("../../red.gb", 'rb')
f.seek(0x34000)
#f.seek(0x34162)
dcmp = Decompressor(f)
out = untile(dcmp, dcmp.decompress())
#out = untile_mirror(out)
savepgm(dcmp, out, sys.stdout)
