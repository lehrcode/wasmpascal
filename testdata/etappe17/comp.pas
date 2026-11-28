{ Comp: Real-Typ, im Speicher eine 64-Bit-Ganzzahl wie in TP }
program CompTest;

type
  TSatz = record
    K: Byte;
    C: Comp
  end;

var
  C, D: Comp;
  I: Int64;
  S: TSatz;
  F: file of Comp;
  B: file of Byte;
  X: Byte;
  K: Integer;

begin
  WriteLn(SizeOf(Comp), ' ', SizeOf(TSatz));
  C := 0.5; Write(C:0:1, ' ');
  C := 1.5; Write(C:0:1, ' ');
  C := 2.5; Write(C:0:1, ' ');
  C := -2.5; Write(C:0:1, ' ');
  C := -3.7; WriteLn(C:0:1);
  C := 7;
  D := C / 2;                      { Comp / Comp ist Real; gespeichert gerundet }
  WriteLn(C / 2:0:2, ' ', D:0:1);
  WriteLn(C);
  { Int64 <-> Comp ohne Umweg über Real }
  I := 9007199254740993;           { 2^53 + 1 }
  C := I;
  I := 0;
  I := C;
  WriteLn(I);
  { außerhalb: "indefinite" }
  C := 1E30;
  I := C;
  WriteLn(I = Low(Int64));
  { als Ganzzahl in Record und Datei }
  S.K := 1;
  S.C := 258;
  Assign(F, 'comp.dat');
  Rewrite(F);
  Write(F, S.C);
  Close(F);
  Assign(B, 'comp.dat');
  Reset(B);
  for K := 1 to 8 do
  begin
    Read(B, X);
    Write(X, ' ')
  end;
  WriteLn;
  Close(B)
end.
