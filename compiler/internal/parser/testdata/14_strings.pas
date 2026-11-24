program Zeichenketten;
const
  CRLF = #13#10;
  Glocke = ^G;
  Gruss = 'Grüß Gott';
var
  s, t: string;
  kurz: string[5];
  i, p: Integer;
begin
  s := 'Turbo' + ' ' + 'Pascal';
  kurz := s;
  WriteLn(kurz, ' / ', Length(s), ' / ', Ord(s[0]));
  t := Copy(s, 7, 6);
  p := Pos('Pascal', s);
  Delete(s, 1, 6);
  Insert('Borland ', s, 1);
  s := Concat(s, '!', CRLF);
  for i := 1 to Length(t) do
    t[i] := UpCase(t[i]);
  WriteLn(t, p);
  WriteLn('Er sagte: ''Hallo''', Glocke);
  WriteLn(Gruss, ' hat ', Length(Gruss), ' Bytes');
  Str(3.14159:6:2, s);
  Val('123', i, p);
  WriteLn('Zeile 1'#13#10'Zeile 2', ^M^J'Zeile 3')
end.
