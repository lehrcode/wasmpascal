{ Der Typ Int64 (aus Delphi 4) }
program Ganz64;

var
  A, B, C: Int64;
  L: LongInt;
  W: Word;
  R: Real;
  I: Int64;
  S: string;
  Code: Integer;
  F: file of Int64;

function Fakultaet(N: Integer): Int64;
var
  K: Integer;
begin
  Result := 1;
  for K := 2 to N do
    Result := Result * K
end;

begin
  A := High(Int64);
  B := Low(Int64);
  WriteLn(A);
  WriteLn(B);
  WriteLn(A + 1 = B);                   { Überlauf ohne Prüfung }
  WriteLn(SizeOf(Int64), ' ', SizeOf(A));

  { gemischt mit LongInt }
  L := MaxLongInt;
  A := L + 1;                           { LongInt + LongInt: 32 Bit }
  WriteLn(A);
  A := L;
  A := A + 1;                           { Int64 + LongInt: 64 Bit }
  WriteLn(A);
  C := 3000000000;                      { Dezimalkonstante über MaxLongInt }
  WriteLn(C, ' ', C * 3);
  C := $100000000;                      { 9 Hexziffern: Int64 }
  WriteLn(C, ' ', $FFFFFFFF, ' ', $FFFFFFFFFFFFFFFF);
  L := C + 5;                           { abgeschnitten }
  W := C - 1;
  WriteLn(L, ' ', W);
  R := C;
  WriteLn(R:0:0);

  { Arithmetik und Bitoperationen }
  A := 1000000007;
  B := A * A;
  WriteLn(B, ' ', B div 7, ' ', B mod 1000, ' ', -B);
  WriteLn(B and $FFFF, ' ', B or 1, ' ', B xor B, ' ', not Int64(0));
  WriteLn(Int64(1) shl 40, ' ', B shr 60, ' ', Int64(1) shl 64);
  WriteLn(Abs(-B), ' ', Sqr(Int64(3000000000)), ' ', Odd(B));
  WriteLn(Fakultaet(20));
  WriteLn(B > A, ' ', B < A, ' ', A = 1000000007, ' ', A <> L);
  A := 5;
  Inc(A, 10000000000);
  Dec(A);
  WriteLn(A, ' ', Succ(A), ' ', Pred(A), ' ', Ord(A));
  for I := 4999999998 to 5000000001 do
    Write(I, ' ');
  WriteLn;
  for I := 2 downto 0 do
    Write(I, ' ');
  WriteLn;

  { Ein- und Ausgabe }
  WriteLn('[', A:15, ']', '[', -A:1, ']');
  Str(B, S);
  WriteLn(S, ' ', Length(S));
  Str(Low(Int64):25, S);
  WriteLn('[', S, ']');
  Val('-9223372036854775808', A, Code);
  WriteLn(A, ' ', Code);
  Val('9223372036854775808', A, Code);
  WriteLn(Code);
  Val('$7FFFFFFFFFFFFFFF', A, Code);
  WriteLn(A, ' ', Code);
  Val('12x', A, Code);
  WriteLn(Code);
  ReadLn(A, B);
  WriteLn(A + B);

  Assign(F, 'zahlen.dat');
  Rewrite(F);
  Write(F, Low(Int64), High(Int64), Int64(42));
  Close(F);
  Reset(F);
  WriteLn(FileSize(F));
  while not Eof(F) do
  begin
    Read(F, A);
    Write(A, ' ')
  end;
  WriteLn;
  Close(F);

  { Division durch 0 }
  A := 0;
  WriteLn(B div A)
end.
