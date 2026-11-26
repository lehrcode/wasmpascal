program Lesen;
{ Read einzelner WideChar aus UTF-8: Umlaut, Euro, Emoji (einmal #$FFFD),
  ungültiges Byte, Zeilenende #10, Dateiende #26. }
var
  W: WideChar;
  N: Integer;
begin
  N := 0;
  repeat
    Read(W);
    Inc(N);
    Write(Ord(W), ' ')
  until (W = #10) or (N > 20);
  WriteLn;
  Read(W);
  WriteLn(Ord(W));
  ReadLn;
  Read(W);
  WriteLn(Ord(W))
end.
