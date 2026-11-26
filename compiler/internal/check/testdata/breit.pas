program Breit;
type
  TBuchstabe = 'a'..'z';
  TBreit = #$100..#$17F;
  TMisch = 'a'..#$20AC;
  TKurz = -1000..1000;
  TRec = record
    C: Char;
    W: WideChar;
    S: SmallInt;
  end;
const
  Euro = #$20AC;
  Ae: WideChar = 'ä';
  Klein: SmallInt = -32768;
  Text_ = 'x'#$20AC;
var
  W: WideChar;
  S: SmallInt;
  Wort: Word;
  R: TRec;
begin
end.
