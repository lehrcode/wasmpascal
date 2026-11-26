program Utf;
{ UTF8Length, UTF8Copy, UTF8Index, UTF8Next und for-in mit WideChar und
  Char; ungültige Bytes zählen je Byte als ein Zeichen (#$FFFD). }
var
  S, T: string;
  I: Integer;
  W: WideChar;
  C: Char;
begin
  S := 'Grüße, 5€!';
  WriteLn(Length(S), ' ', UTF8Length(S));
  WriteLn(UTF8Copy(S, 3, 3), '|', UTF8Copy(S, 9, 10), '|', UTF8Copy(S, 0, 2), '|', UTF8Copy(S, 20, 1), '|');
  WriteLn(UTF8Index(S, 1), ' ', UTF8Index(S, 4), ' ', UTF8Index(S, 10), ' ', UTF8Index(S, 11), ' ', UTF8Index(S, 0));
  I := 1;
  while I <= Length(S) do
  begin
    W := UTF8Next(S, I);
    Write(Ord(W), ' ')
  end;
  WriteLn;
  I := 0; W := UTF8Next(S, I); WriteLn(Ord(W), ' ', I);
  I := 15; W := UTF8Next(S, I); WriteLn(Ord(W), ' ', I);
  T := 'a' + #$FF + 'b' + #$C3;
  WriteLn(UTF8Length(T));
  for W in T do
    Write(Ord(W), ' ');
  WriteLn;
  for C in T do
    Write(Ord(C), ' ');
  WriteLn;
  T := '😀x';
  WriteLn(Length(T), ' ', UTF8Length(T), ' ', UTF8Copy(T, 2, 1));
  for W in T do
    Write(Ord(W), ' ');
  WriteLn;
  T := #$C0#$AF;
  WriteLn(UTF8Length(T));
  T := #$E2#$82;
  WriteLn(UTF8Length(T));
  WriteLn(UTF8Length(''), ' ', UTF8Index('', 1), ' [', UTF8Copy('', 1, 1), ']');
  for W in 'Öl' do
    Write(W, '.');
  WriteLn
end.
