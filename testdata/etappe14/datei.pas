program BreitDatei;
{ file of WideChar speichert 2 Bytes je Zeichen. }
var
  F: file of WideChar;
  U: file;
  W: WideChar;
begin
  Assign(F, 'w.dat');
  Rewrite(F);
  W := 'ä'; Write(F, W);
  W := #$20AC; Write(F, W);
  W := 'z'; Write(F, W);
  Close(F);
  Reset(F);
  WriteLn(FileSize(F));
  while not Eof(F) do
  begin
    Read(F, W);
    Write(Ord(W), ' ')
  end;
  WriteLn;
  Close(F);
  Assign(U, 'w.dat');
  Reset(U, 1);
  WriteLn(FileSize(U));
  Close(U)
end.
