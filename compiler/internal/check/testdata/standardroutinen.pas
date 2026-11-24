program Standardroutinen;
{ Ruft jede Standardroutine der System-Unit mindestens einmal auf. }
type
  PRec = ^TRec;
  TRec = record a, b: Integer end;
var
  i, code: Integer;
  w: Word;
  l: LongInt;
  r: Real;
  c: Char;
  b: Boolean;
  s: string;
  t: Text;
  f: file of TRec;
  u: file;
  p: PRec;
  q: Pointer;
  rec: TRec;
  buf: array[1..128] of Byte;
begin
  Write('a', 1, 2.5, True, 'x':3, 1.5:8:2);
  WriteLn;
  WriteLn(t, 'Datei');
  Write(Output, i);
  Read(i);
  ReadLn(t, s, c, r);
  ReadLn;
  Inc(i); Inc(i, 2); Dec(c); Dec(l, i);
  New(p); Dispose(p);
  GetMem(q, 100); FreeMem(q, 100);
  Mark(q); Release(q);
  Assign(t, 'x.txt'); Reset(t); Rewrite(t); Append(t); Flush(t); Close(t);
  Assign(f, 'f.dat'); Reset(f); Write(f, rec); Read(f, rec); Seek(f, 0); Truncate(f);
  l := FilePos(f) + FileSize(f);
  Erase(f); Rename(f, 'g.dat');
  Assign(u, 'u.dat'); Reset(u, 1); Rewrite(u, 128);
  BlockRead(u, buf, 1, w); BlockWrite(u, buf, 1);
  b := Eof(t) or Eoln(t) or SeekEof(t) or SeekEoln(t) or Eof or Eoln;
  i := IOResult;
  Delete(s, 1, 2); Insert('ab', s, 1);
  Str(i, s); Str(r:6:2, s); Val(s, r, code); Val('12', i, code);
  FillChar(buf, SizeOf(buf), 0); Move(buf, rec, SizeOf(rec));
  Randomize;
  ChDir('/'); MkDir('neu'); RmDir('neu'); GetDir(0, s);
  r := Abs(r) + ArcTan(1) + Cos(0) + Sin(0) + Exp(1) + Ln(2) + Sqrt(4) + Frac(r) + Int(r) + Sqr(r) + Pi + Random;
  l := Abs(i) + Sqr(i) + Trunc(r) + Round(r) + Random(10);
  c := Chr(65); l := Ord(c); b := Odd(i); c := Succ(c); c := Pred(c);
  i := Hi(w) + Lo(w) + Swap(i) + High(i) + Low(w) + SizeOf(TRec);
  i := Length(s) + Pos('a', s);
  s := Copy(s, 1, 3) + Concat(s, 'x', c);
  c := UpCase(c);
  l := MemAvail + MaxAvail + ParamCount;
  s := ParamStr(0);
  q := Ptr(0, 100); q := Addr(i); l := Ofs(i) + Seg(i);
  RandSeed := 42; FileMode := 2; ExitCode := 0;
  if i > 0 then Exit;
  while True do begin Break; Continue end;
  Halt; Halt(1)
end.
