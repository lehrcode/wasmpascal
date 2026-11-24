program Zeichenketten;
type
  TName = string[5];
var
  s, t: string;
  k: TName;
  c: Char;
  i, n: Integer;

function Spiegel(const s: string): string;
var
  i: Integer;
begin
  Result := '';
  for i := Length(s) downto 1 do
    Result := Result + s[i]
end;

procedure Anhaengen(var s: string; const t: string);
begin
  s := s + t
end;

procedure Wert(s: string);
begin
  s := s + '!';
  Write(s, ' ')
end;

begin
  s := 'Turbo';
  t := s + ' ' + 'Pascal';
  WriteLn(t, ' ', Length(t), ' ', Ord(t[0]));
  k := t;
  WriteLn(k, ' ', Length(k));
  k := 'ab';
  k := k + 'cdefgh';
  WriteLn(k);
  WriteLn(Copy(t, 7, 6), '|', Copy(t, 7, 100), '|', Copy(t, 50, 2), '|');
  WriteLn(Pos('Pascal', t), ' ', Pos('x', t), ' ', Pos('', t));
  s := t;
  Delete(s, 1, 6);
  WriteLn(s);
  Insert('Borland ', s, 1);
  WriteLn(s);
  Insert('!', s, 100);
  WriteLn(s);
  k := 'abc';
  Insert('XYZ', k, 2);
  WriteLn(k);
  s := 'Hallo';
  s[1] := 'B';
  s[Length(s)] := UpCase(s[Length(s)]);
  WriteLn(s);
  WriteLn('abc' < 'abd', ' ', 'abc' < 'ab', ' ', 'B' > 'A', ' ', s = 'BallO', ' ', 'x' = s);
  c := 'z';
  s := c;
  s := s + c + 'y';
  WriteLn(s, ' ', Length(s));
  WriteLn(Spiegel('Lager'), ' ', Spiegel(''), '|');
  s := 'Wert';
  Wert(s);
  WriteLn(s);
  Anhaengen(s, 'e');
  WriteLn(s);
  n := 0;
  for c in 'Größe' do Inc(n);
  WriteLn(n, ' ', Length('Größe'));
  s := Concat('a', 'b', 'c', s);
  WriteLn(s);
  t := '';
  for i := 1 to 300 do t := t + 'x';
  WriteLn(Length(t));
  WriteLn('Er sagte: ''Hallo''')
end.
