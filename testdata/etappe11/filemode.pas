program Zugriff;
{ FileMode bei Reset typisierter und untypisierter Dateien:
  0 = nur lesen, 1 = nur schreiben, 2 = beides (Standard). Lesen bzw.
  Schreiben gegen die Zugriffsart ergibt E/A-Fehler 5. Rewrite und
  Textdateien beachten FileMode nicht. }
var
  F: file of Integer;
  U: file;
  T: Text;
  I, N: Integer;
  Puffer: array[1..4] of Byte;
  S: string;
begin
  WriteLn('FileMode: ', FileMode);
  Assign(F, 'zahlen.dat');
  Rewrite(F);
  for I := 1 to 3 do
    Write(F, I);
  Close(F);

  FileMode := 0;
  Reset(F);
  Read(F, I);
  WriteLn('nur lesen: ', I);
  {$I-} Write(F, I); {$I+} N := IOResult;
  WriteLn('schreiben: ', N);
  {$I-} Truncate(F); {$I+} N := IOResult;
  WriteLn('Truncate: ', N);
  WriteLn('Groesse: ', FileSize(F));
  Close(F);

  FileMode := 1;
  Reset(F);
  {$I-} Read(F, I); {$I+} N := IOResult;
  WriteLn('lesen: ', N);
  Seek(F, 2);
  I := 30;
  Write(F, I);
  Close(F);

  FileMode := 0;
  Assign(U, 'zahlen.dat');
  Reset(U, 4);
  WriteLn('Saetze: ', FileSize(U));
  Seek(U, 2);
  BlockRead(U, Puffer, 1);
  WriteLn('Satz 2: ', Puffer[1]);
  {$I-} BlockWrite(U, Puffer, 1); {$I+} N := IOResult;
  WriteLn('BlockWrite: ', N);
  Close(U);

  { Rewrite ignoriert FileMode: lesen und schreiben }
  Rewrite(F);
  I := 7;
  Write(F, I);
  Seek(F, 0);
  Read(F, I);
  WriteLn('Rewrite liest: ', I);
  Close(F);

  { Textdateien ignorieren FileMode }
  FileMode := 1;
  Assign(T, 'text.txt');
  Rewrite(T); WriteLn(T, 'Text'); Close(T);
  Reset(T); ReadLn(T, S); Close(T);
  WriteLn('Text: ', S);

  FileMode := $42;   { Sharing-Bits werden ignoriert: wie 2 }
  Reset(F);
  I := 8;
  Write(F, I);
  Seek(F, 0);
  Read(F, I);
  WriteLn('FileMode $42: ', I);
  Close(F)
end.
